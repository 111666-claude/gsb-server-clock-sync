# server-clock-sync

时钟校正器：按来源分组采样、估计偏移，再把拨动量按整秒预算下发。命令行走 `flag` 加环境变量
（`SYNC_WINDOW_MS` 覆盖窗口），输出一行 JSON，只用 Go 标准库。

```
go run ./cmd/clock-sync --sample=median
go run ./cmd/clock-sync --sample=window
go run ./cmd/clock-sync --sample=jump
go run ./cmd/clock-sync --sample=budget
go run ./cmd/clock-sync --sample=work
go test ./...
go vet ./...
```

## 口径（README 为准）

- **按来源分组**：每个来源各自维护样本与已下发值，互不影响；`Add` 只记录不估计。
- **采样窗口**：只用距 `nowMs` 不超过 `windowMs` 的样本，正好等于边界算在内，窗口外的必须回收。
- **偏移估计**：每次握手偏移是 `((t2-t1)+(t3-t4))/2`，窗口内取中位数（偶数个取中间偏左那个）；
  平均 RTT 取窗口内 `(t4-t1)-(t3-t2)` 的算术平均；窗口内样本少于 `minSamples` 返回 `ErrNoSamples`。
- **突变抑制**：新估计与已下发值之差超过 `maxJump` 时不下发并计入 `Blocked`。
- **单调**：拨动量等于新估计减已下发值；差为负时按 0 处理，时间只许往前拨。
- **整秒预算**：同一整秒内累计拨动不超过 `budgetMs`，放不下的部分截断并计入 `Deferred`，跨秒重新计算。
- **幂等**：同一个 (来源, 时刻) 只下发一次，重复调用返回 `(0, false)`。
- **代价**：估计不许全扫全部样本，`Scanned` 不随样本数乘调用次数增长；每秒 10 万次估计。

## 输出契约（不改格式）

```
{"offset":..}
{"ok":..,"blocked":..}
{"second":..,"deferred":..}
{"scanned":..}
```

## 目录

```
clocksync/sync.go            分组采样、中位数、突变抑制、单调与预算
cmd/clock-sync/main.go       命令行入口（flag 加环境变量，JSON 输出）
clocksync/sync_test.go       单元用例
```
