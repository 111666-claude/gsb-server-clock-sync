// Package clocksync 是时钟校正器：按来源分组采样、估计偏移，再把拨动量按整秒预算下发。
// 缺陷：采样窗口从不回收、偏移取平均而不是中位数、没有突变抑制、下发不单调、
// 整秒预算不生效、同一个 (来源, 时刻) 会重复下发、估计时全扫所有样本。
package clocksync

import "sort"

// ErrNoSamples 样本不足。
type ErrNoSamples struct{}

func (ErrNoSamples) Error() string { return "样本不足" }

type sample struct {
	offset int
	rtt    int
	atMs   int
}

// Sync 是时钟校正台账。
type Sync struct {
	windowMs   int
	minSamples int
	maxJump    int
	budgetMs   int
	samples    map[string][]sample
	applied    map[string]int
	spent      map[string]int
	ticks      map[string]bool
	Blocked    int
	Deferred   int
	scanned    int
}

// New 建台账：windowMs 是采样窗口，minSamples 是最少样本数，
// maxJump 是一次允许的最大拨动量，budgetMs 是每秒拨动预算。
func New(windowMs, minSamples, maxJump, budgetMs int) *Sync {
	return &Sync{
		windowMs: windowMs, minSamples: minSamples, maxJump: maxJump, budgetMs: budgetMs,
		samples: map[string][]sample{}, applied: map[string]int{},
		spent: map[string]int{}, ticks: map[string]bool{},
	}
}

// Add 记录一次四步握手：t1、t2 是本地发出与服务器收到，t3、t4 是服务器发出与本地收到。
// 缺陷：不裁窗口，全扫样本。
func (s *Sync) Add(source string, t1, t2, t3, t4, atMs int) {
	s.samples[source] = append(s.samples[source], sample{
		offset: ((t2 - t1) + (t3 - t4)) / 2,
		rtt:    (t4 - t1) - (t3 - t2),
		atMs:   atMs,
	})
}

// Offset 返回该来源的偏移估计与平均 RTT。缺陷：窗口不裁、取平均、样本不足也返回、全扫样本。
func (s *Sync) Offset(source string, nowMs int) (int, int, error) {
	s.scanned += len(s.samples[source])
	items := s.samples[source]
	if len(items) == 0 {
		return 0, 0, ErrNoSamples{}
	}
	total := 0
	rtt := 0
	for _, item := range items {
		total += item.offset
		rtt += item.rtt
	}
	return total / len(items), rtt / len(items), nil
}

// Tick 下发一次校正，返回本次实际拨动的毫秒数。缺陷：不查幂等、不查突变、不单调、不看整秒预算。
func (s *Sync) Tick(source string, nowMs int) (int, bool) {
	offset, _, err := s.Offset(source, nowMs)
	if err != nil {
		return 0, false
	}
	delta := offset - s.applied[source]
	s.applied[source] = offset
	s.ticks[source] = true
	return delta, true
}

// DropSamples 清掉某来源的样本，用于模拟网络切换后的新窗口。
func (s *Sync) DropSamples(source string) {
	s.samples[source] = nil
}

// Sorted 返回排序副本，供调用方做中位数之类的参考计算。
func Sorted(values []int) []int {
	out := append([]int{}, values...)
	sort.Ints(out)
	return out
}

// Scanned 是累计扫描的样本数（规模观测）。
func (s *Sync) Scanned() int { return s.scanned }
