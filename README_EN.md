# peakewma

> 中文文档: [README.md](./README.md)

`peakewma` is a load-balancer selector implementation based on **Peak EWMA (Exponentially Weighted Moving Average)**, designed for high-concurrency service calls.

It combines:

- latency EWMA
- in-flight requests (pending)
- per-instance QPS
- instance health (from business error codes)

This helps shift traffic away from overloaded or degraded backends faster, improving throughput and stability.

---

## Features

- **Latency-aware routing**: Continuously tracks per-instance EWMA latency and favors lower scores.
- **Pending pressure control**: Penalizes instances with more active requests.
- **QPS biasing**: Applies exponential penalty when instance QPS is significantly above average.
- **Health decay/recovery**: Supports demotion on configured error codes and natural recovery over time.
- **State cleanup**: Removes stale instance state after timeout to prevent unbounded growth.

---

## Installation

```bash
go get github.com/Huafanfan/peakewma
```

---

## Quick Start

### 1) Initialize config and manager

```go
package main

import (
	"time"

	"github.com/Huafanfan/peakewma"
	"github.com/Huafanfan/peakewma/config"
)

func main() {
	cfg := config.NewConfig()
	cfg.Timeout = config.TimeDuration(5 * time.Minute)
	cfg.ErrorCodeList = []uint32{50001, 50002}

	lb := peakewma.NewPeakEWMAManager(cfg)
	_ = lb
}
```

### 2) Select an instance

```go
instances := []*peakewma.ServiceInstance{
	{ID: "a"},
	{ID: "b"},
	{ID: "c"},
}

chosen := lb.Select(instances)
if chosen == nil {
	// no available instance
}
```

### 3) Report request lifecycle

```go
// before request
lb.Update(peakewma.Record{
	InstanceID: chosen.ID,
	T:          peakewma.StartPendingEWMA,
})

// ... execute RPC/HTTP call ...

// after request (example: 40ms, success)
lb.Update(peakewma.Record{
	InstanceID: chosen.ID,
	T:          peakewma.FinishPendingEWMA,
	Latency:    int64((40 * time.Millisecond).Microseconds()),
	Err:        0, // if Err in ErrorCodeList -> demotion error
})
```

### 4) Run periodic tasks

```go
ticker := time.NewTicker(time.Second)
defer ticker.Stop()

for range ticker.C {
	lb.Tick()  // advance EWMA decay and refresh qpsLastSecond
	lb.Clean() // remove stale instance state
}
```

---

## Scoring Model (Simplified)

Lower score is preferred:

```text
score = durationEWMA * pendingBias * qpsBias
```

- `durationEWMA`: latency EWMA in microseconds; falls back to `DefaultPeakEWMADuration` when empty
- `pendingBias`: `(activeRequests + 1) ^ ActiveRequestBias`
- `qpsBias`: `max(1, (instanceQPS / avgQPS) ^ QPSBias)`

You can disable dimensions independently via:
`EnableDuration`, `EnablePending`, `EnableQPS`, `EnableHealth`.

---

## Config Fields

Main fields in `config.Config`:

- `Timeout`: stale state cleanup timeout
- `Alpha`: EWMA smoothing factor
- `DefaultPeakEWMADuration`: fallback duration without samples
- `PickTimes`: retry count for random sampling
- `EnableHealth`: enable health gating
- `EnableDuration`: enable duration dimension
- `EnablePending`: enable pending dimension
- `EnableQPS`: enable QPS dimension
- `ErrorCodeList`: error codes treated as demotion failures
- `EnableTick`: whether periodic tick is enabled (driven by caller)
- `ActiveRequestBias`: exponent for pending penalty
- `QPSBias`: exponent for QPS penalty

---

## Operational Notes

- Run `Tick()` every second (recommended), then call `Clean()` in the same loop.
- Keep `StartPendingEWMA` and `FinishPendingEWMA` calls paired to avoid skewed stats.
- This package is a good fit as an embedded instance selector in gateways/RPC SDKs/service discovery layers.

---

## Development & Testing

```bash
go test ./...
```
