# server-clock-sync

服务器时钟同步：用四步握手样本估计本地与服务器的偏移。命令行走 `flag` 加环境变量，输出一行 JSON，
只用 Go 标准库。

```
go run ./cmd/clock-sync --sample=median
go run ./cmd/clock-sync --sample=window
go run ./cmd/clock-sync --sample=monotonic
go run ./cmd/clock-sync --sample=work
SYNC_WINDOW_MS=100 go run ./cmd/clock-sync --window=1000 --sample=window
go test ./...
go vet ./...
```

## 口径（README 为准）

- **采样窗口**：只用距 `nowMs` 不超过 `windowMs` 的样本，正好等于窗口边界算在内，窗口外的样本必须回收。
- **偏移估计**：每次握手的偏移是 `((t2-t1)+(t3-t4))/2`，窗口内取这些偏移的中位数（偶数个取中间偏左那个）。
- **平均 RTT**：窗口内 `(t4-t1)-(t3-t2)` 的算术平均。
- **单调**：返回的偏移估计不得小于上一次的估计值（时钟只许往前拨），回退时保持上一次的值。
- **样本不足**：窗口内样本数少于 `minSamples` 时返回 `ErrNoSamples`，不返回任何估计。
- **代价**：估计不许全扫全部样本，`Scanned` 不随样本数乘调用次数增长；每秒 10 万次估计。

## 输出契约（不改格式）

```
{"offset":..}
{"scanned":..}
```

## 目录

```
clocksync/sync.go            窗口、中位数与单调估计
cmd/clock-sync/main.go       命令行入口（flag 加环境变量，JSON 输出）
clocksync/sync_test.go       单元用例
```
