package peakewma

import (
	"math"
	"time"

	uberAtomic "go.uber.org/atomic"
)

const (
	minDecayDuration = time.Second
)

// EWMA is the standard EWMA implementation, it updates in real time
// and when queried it always returns the decayed value.
// See: https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average
type EWMA struct {
	alpha float64

	lastEWMA     uberAtomic.Float64
	lastTickTime time.Time
}

// NewEWMA constructs a new EWMA with the given alpha.
func NewEWMA(alpha float64) *EWMA {
	ewma := &EWMA{
		alpha: alpha,
	}
	ewma.lastEWMA.Store(0.0)
	return ewma
}

// Rate returns the moving average mean of events per second.
func (e *EWMA) Rate() float64 {
	return e.lastEWMA.Load()
}

// Tick ticks the clock and use lastEWMA to update the moving average.
func (e *EWMA) Tick() {
	// After update, lastEWMA = decay * lastEWMA
	e.Update(0)
}

// Update adds an uncounted event with value `i`, and tries to flush.
func (e *EWMA) Update(i int64) {
	now := time.Now()

	lastEWMA := e.lastEWMA.Load()
	lastTickTime := e.lastTickTime
	e.lastEWMA.Store(e.ewma(lastEWMA, float64(i), lastTickTime, now))
	e.lastTickTime = now
}

func (e *EWMA) ewma(lastEWMA, i float64, lastTickTime, now time.Time) float64 {
	td := float64(now.Sub(lastTickTime)) / float64(time.Second)
	if td < 0 {
		td = 0
	}
	decay := math.Pow(1-e.alpha, td)
	return i*(1-decay) + decay*lastEWMA
}
