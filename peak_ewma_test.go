package peakewma

import (
	"testing"
	"time"

	"github.com/Huafanfan/peakewma/config"
)

func TestScoreUsesFloatQPSRatio(t *testing.T) {
	cfg := config.NewConfig()
	cfg.EnableDuration = false
	cfg.EnablePending = false
	cfg.EnableQPS = true
	cfg.QPSBias = 2
	cfg.DefaultPeakEWMADuration = config.TimeDuration(2 * time.Second)

	ins := newInstanceEWMA(cfg.Alpha)
	ins.qpsLastSecond.Store(150)

	score := ins.score(cfg, 100)
	expected := float64((2 * time.Second).Microseconds()) * 2.25
	if score != expected {
		t.Fatalf("unexpected score, got=%v expected=%v", score, expected)
	}
}

func TestUpdateFinishCreatesInstanceAndNoNegativePending(t *testing.T) {
	cfg := config.NewConfig()
	manager := NewPeakEWMAManager(cfg)

	manager.Update(Record{
		InstanceID: "ins-a",
		T:          FinishPendingEWMA,
		Latency:    int64((50 * time.Millisecond).Microseconds()),
		Err:        0,
	})

	v, ok := manager.instanceEWMAStore.Load("ins-a")
	if !ok {
		t.Fatalf("expected instance state to be initialized")
	}
	state := v.(*instanceEWMA)
	if pending := state.upstreamRequestActive(); pending != 0 {
		t.Fatalf("pending requests should not be negative, got=%d", pending)
	}
	if rate := state.upstreamRequestDurationEWMA.Rate(); rate <= 0 {
		t.Fatalf("duration ewma should be updated, got=%v", rate)
	}
}
