package peakewma

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Huafanfan/peakewma/config"
	uberAtomic "go.uber.org/atomic"
	"pgregory.net/rand"
)

const (
	failScore       = 10
	throttleSuccess = 5
	successScore    = 0
)

type peakEWMAManager struct {
	cfg               *config.Config
	alpha             float64
	instanceEWMAStore sync.Map // instanceID -> *instanceEWMA
}

func NewPeakEWMAManager(cfg *config.Config) *peakEWMAManager {
	p := &peakEWMAManager{
		cfg:   cfg,
		alpha: cfg.Alpha,
	}
	return p
}

func (p *peakEWMAManager) peakEWMA(ins []*ServiceInstance) *ServiceInstance {
	var selectedA, selectedB int
	var selectedAHealthy, selectedBHealthy bool

	switch len(ins) {
	case 1:
		return ins[0]
	case 2:
		selectedA = 0
		selectedAHealthy = p.instanceHealthy(ins[selectedA])
		selectedB = 1
		selectedBHealthy = p.instanceHealthy(ins[selectedB])
	default:
		for i := 0; i < p.cfg.PickTimes; i++ {
			selectedA = rand.Intn(len(ins))
			selectedAHealthy = p.instanceHealthy(ins[selectedA])
			if selectedAHealthy {
				break
			}
		}
		for i := 0; i < p.cfg.PickTimes; i++ {
			selectedB = (rand.Intn(len(ins)-1) + selectedA + 1) % len(ins)
			selectedBHealthy = p.instanceHealthy(ins[selectedB])
			if selectedBHealthy {
				break
			}
		}
	}

	if selectedAHealthy && selectedBHealthy {
		return p.choose(ins[selectedA], ins[selectedB], (p.instanceQPS(ins[selectedA])+p.instanceQPS(ins[selectedB]))/2)
	} else if selectedAHealthy {
		return ins[selectedA]
	} else if selectedBHealthy {
		return ins[selectedB]
	} else {
		return p.choose(ins[selectedA], ins[selectedB], 0)
	}
}

func (p *peakEWMAManager) choose(insA *ServiceInstance, insB *ServiceInstance, approximateAverageQPS int64) *ServiceInstance {
	if p.peakEWMAScore(insA, approximateAverageQPS) < p.peakEWMAScore(insB, approximateAverageQPS) {
		return insA
	}
	return insB
}

func (p *peakEWMAManager) instanceQPS(ins *ServiceInstance) int64 {
	v, ok := p.instanceEWMAStore.Load(ins.ID)
	if !ok {
		return 0
	}
	return v.(*instanceEWMA).qpsLastSecond.Load()
}

func (p *peakEWMAManager) instanceHealthy(ins *ServiceInstance) bool {
	if !p.cfg.EnableHealth {
		return true
	}
	v, ok := p.instanceEWMAStore.Load(string(ins.ID))
	if !ok {
		return true
	}
	return v.(*instanceEWMA).healthy()
}

func (p *peakEWMAManager) peakEWMAScore(ins *ServiceInstance, approximateAverageQPS int64) float64 {
	v, ok := p.instanceEWMAStore.Load(ins.ID)
	if !ok {
		return 0
	}
	return v.(*instanceEWMA).score(p.cfg, approximateAverageQPS)
}

func (p *peakEWMAManager) update(r Record) {
	v, ok := p.instanceEWMAStore.Load(r.InstanceID)
	if !ok {
		c := newInstanceEWMA(p.alpha)
		if r.T == StartPendingEWMA {
			c.startPending()
			p.instanceEWMAStore.Store(r.InstanceID, c)
		}
	} else {
		if r.T == StartPendingEWMA {
			v.(*instanceEWMA).startPending()
		} else {
			v.(*instanceEWMA).finishPending()
			v.(*instanceEWMA).update(r, p.cfg)
		}
	}
}

func (p *peakEWMAManager) tick() {
	p.instanceEWMAStore.Range(func(k interface{}, v interface{}) bool {
		v.(*instanceEWMA).upstreamRequestDurationEWMA.Tick()
		v.(*instanceEWMA).upstreamInstanceHealthEWMA.Update(throttleSuccess - 1)
		// update instance qps
		v.(*instanceEWMA).qpsLastSecond.Store(v.(*instanceEWMA).qps.Load())
		v.(*instanceEWMA).qps.Store(0)
		return true
	})
}

func (p *peakEWMAManager) clean() {
	p.instanceEWMAStore.Range(func(k interface{}, v interface{}) bool {
		if time.Since(v.(*instanceEWMA).lastAccessTime.Load()) >= time.Duration(p.cfg.Timeout) {
			p.instanceEWMAStore.Delete(k)
		}
		return true
	})
}

type instanceEWMA struct {
	qps           atomic.Int64
	qpsLastSecond atomic.Int64

	pendingRequests             *uberAtomic.Int64
	lastAccessTime              *uberAtomic.Time
	upstreamRequestDurationEWMA *EWMA
	upstreamInstanceHealthEWMA  *EWMA
}

func newInstanceEWMA(alpha float64) *instanceEWMA {
	return &instanceEWMA{
		pendingRequests:             uberAtomic.NewInt64(0),
		upstreamRequestDurationEWMA: NewEWMA(alpha),
		upstreamInstanceHealthEWMA:  NewEWMA(alpha),
		lastAccessTime:              uberAtomic.NewTime(time.Now()),
	}
}

// StartPending ...
func (p *instanceEWMA) startPending() {
	p.qps.Add(1)
	p.pendingRequests.Add(1)
	p.lastAccessTime.Store(time.Now())
}

// FinishPending ...
func (p *instanceEWMA) finishPending() {
	p.pendingRequests.Add(-1)
	p.lastAccessTime.Store(time.Now())
}

func (p *instanceEWMA) upstreamRequestActive() int64 {
	return p.pendingRequests.Load()
}

func (p *instanceEWMA) healthy() bool {
	return p.upstreamInstanceHealthEWMA.Rate() < throttleSuccess
}

func (p *instanceEWMA) score(c *config.Config, approximateAverageQPS int64) float64 {
	duration := float64(time.Duration(c.DefaultPeakEWMADuration).Microseconds())
	if c.EnableDuration {
		duration = p.upstreamRequestDurationEWMA.Rate()
		if duration == 0 {
			duration = float64(time.Duration(c.DefaultPeakEWMADuration).Microseconds())
		}
	}
	biasedActiveRequest := 1.0
	if c.EnablePending {
		biasedActiveRequest = math.Pow(float64(p.upstreamRequestActive())+1, c.ActiveRequestBias)
	}

	biasedQPS := 1.0
	if c.EnableQPS {
		if approximateAverageQPS != 0 {
			// lastQPS = 100, averageQPS = 100, biasedQPS = 1
			// lastQPS = 80, averageQPS = 100, biasedQPS = 1
			// lastQPS = 150, averageQPS = 100, biasedQPS = 57.66
			// lastQPS = 200, averageQPS = 100, biasedQPS = 1024
			// lastQPS = 300, averageQPS = 100, biasedQPS = 59049
			biasedQPS = math.Max(1.0, math.Pow(float64(p.qpsLastSecond.Load()/approximateAverageQPS), c.QPSBias))
		}
	}

	return duration * biasedActiveRequest * biasedQPS
}

func (p *instanceEWMA) update(r Record, cfg *config.Config) {
	if p.isDemotionError(r.Err, cfg) {
		p.upstreamInstanceHealthEWMA.Update(failScore)
	} else {
		p.upstreamInstanceHealthEWMA.Update(successScore)
	}
	p.upstreamRequestDurationEWMA.Update(r.Latency)
}

func (p *instanceEWMA) isDemotionError(responseCode uint32, cfg *config.Config) bool {
	for _, errorCode := range cfg.ErrorCodeList {
		if responseCode == errorCode {
			return true
		}
	}
	return false
}

type RecordType int

const (
	// StartPendingEWMA ...
	StartPendingEWMA = iota
	// FinishPendingEWMA ...
	FinishPendingEWMA
)

type Record struct {
	InstanceID string
	T          RecordType
	Latency    int64
	Err        uint32
}

type ServiceInstance struct {
	ID string
}
