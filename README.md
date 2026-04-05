# peakewma

> English version: [README_EN.md](./README_EN.md)

`peakewma` 是一个基于 **Peak EWMA（指数加权移动平均）** 思路的负载均衡选择器实现，适用于高并发服务调用场景。它会综合考虑：

- 请求延迟（EWMA）
- 当前实例并发请求数（pending）
- 近一秒请求量（QPS）
- 实例健康状态（业务错误码驱动）

通过这些信号，`peakewma` 能在实例出现慢请求或异常抖动时更快地“避开热点”，提升整体吞吐和稳定性。

---

## 功能概览

- **延迟感知路由**：持续更新每个实例的请求延迟 EWMA，优先选择评分更低的实例。
- **并发压制**：可根据实例正在处理的请求数增加惩罚，避免“雪崩式”压流量到同一节点。
- **QPS 偏置**：当实例瞬时流量明显高于均值时，按指数放大惩罚，帮助流量回归均衡。
- **健康度衰减**：支持基于错误码（如业务失败码）进行健康降级，并自动随时间衰减恢复。
- **状态清理**：定期清理长时间未访问实例，避免状态无限增长。

---

## 安装

```bash
go get github.com/Huafanfan/peakewma
```

---

## 快速开始

### 1) 初始化配置和管理器

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

### 2) 选择实例

```go
instances := []*peakewma.ServiceInstance{
	{ID: "a"},
	{ID: "b"},
	{ID: "c"},
}

chosen := lb.Select(instances)
if chosen == nil {
	// 无可选实例
}
```

### 3) 上报调用生命周期

在发请求前、请求结束后分别上报：

```go
// 请求开始
lb.Update(peakewma.Record{
	InstanceID: chosen.ID,
	T:          peakewma.StartPendingEWMA,
})

// ... 执行 RPC/HTTP 调用 ...

// 请求结束（示例：40ms，成功）
lb.Update(peakewma.Record{
	InstanceID: chosen.ID,
	T:          peakewma.FinishPendingEWMA,
	Latency:    int64((40 * time.Millisecond).Microseconds()),
	Err:        0, // 如果在 ErrorCodeList 中则视为降级错误
})
```

### 4) 周期任务（建议）

```go
ticker := time.NewTicker(time.Second)
defer ticker.Stop()

for range ticker.C {
	lb.Tick()  // 推进 EWMA 衰减、刷新 qpsLastSecond
	lb.Clean() // 清理超时未访问实例
}
```

---

## 评分模型（简化版）

每个实例分数越小越优先：

```text
score = durationEWMA * pendingBias * qpsBias
```

- `durationEWMA`：请求耗时（微秒）EWMA；无样本时使用 `DefaultPeakEWMADuration`
- `pendingBias`：`(activeRequests + 1) ^ ActiveRequestBias`
- `qpsBias`：`max(1, (instanceQPS / avgQPS) ^ QPSBias)`

> 你可以通过开关项（`EnableDuration/EnablePending/EnableQPS/EnableHealth`）关闭任一维度影响。

---

## 配置项说明

`config.Config` 主要字段：

- `Timeout`：实例状态清理超时时间。
- `Alpha`：EWMA 平滑因子（越大越敏感）。
- `DefaultPeakEWMADuration`：无延迟样本时的默认耗时。
- `PickTimes`：随机抽样重试次数。
- `EnableHealth`：是否启用健康度判定。
- `EnableDuration`：是否启用延迟维度。
- `EnablePending`：是否启用并发请求数维度。
- `EnableQPS`：是否启用 QPS 维度。
- `ErrorCodeList`：触发健康降级的错误码集合。
- `EnableTick`：是否启用周期 Tick（由业务决定如何驱动）。
- `ActiveRequestBias`：pending 惩罚指数。
- `QPSBias`：QPS 惩罚指数。

---

## 设计建议

- 建议每秒执行一次 `Tick()`，并在同一个周期中调用 `Clean()`。
- `Update(StartPendingEWMA)` 与 `Update(FinishPendingEWMA)` 要成对调用，避免统计失真。
- 对于网关、RPC SDK、服务发现中间件，可将本库作为“实例选择器”嵌入使用。

---

## 开发与测试

```bash
go test ./...
```
