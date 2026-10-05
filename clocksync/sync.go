// Package clocksync 是时钟校正器：按来源分组采样、估计偏移，再把拨动量按整秒预算下发。
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

// sourceState 是单个来源自己的样本与已下发台账，来源之间互不影响。
type sourceState struct {
	samples []sample // 按 atMs 追加，窗口外的从头部回收
	applied int      // 已下发值
	bucket  int      // 当前整秒桶（nowMs / 1000）
	spent   int      // 当前整秒已用预算
	ticks   map[int]bool

	cacheValid  bool
	cacheOffset int
	cacheRtt    int
}

// Sync 是时钟校正台账。
type Sync struct {
	windowMs   int
	minSamples int
	maxJump    int
	budgetMs   int
	sources    map[string]*sourceState
	Blocked    int
	Deferred   int
	scanned    int
}

// New 建台账：windowMs 是采样窗口，minSamples 是最少样本数，
// maxJump 是一次允许的最大拨动量，budgetMs 是每秒拨动预算。
func New(windowMs, minSamples, maxJump, budgetMs int) *Sync {
	return &Sync{
		windowMs: windowMs, minSamples: minSamples, maxJump: maxJump, budgetMs: budgetMs,
		sources: map[string]*sourceState{},
	}
}

func (s *Sync) state(source string) *sourceState {
	st, ok := s.sources[source]
	if !ok {
		st = &sourceState{ticks: map[int]bool{}}
		s.sources[source] = st
	}
	return st
}

// Add 记录一次四步握手：t1、t2 是本地发出与服务器收到，t3、t4 是服务器发出与本地收到。
// 只记录，不做估计；样本假定按 atMs 先后追加。
func (s *Sync) Add(source string, t1, t2, t3, t4, atMs int) {
	st := s.state(source)
	st.samples = append(st.samples, sample{
		offset: ((t2 - t1) + (t3 - t4)) / 2,
		rtt:    (t4 - t1) - (t3 - t2),
		atMs:   atMs,
	})
	st.cacheValid = false
}

// evict 回收距 nowMs 超过 windowMs 的老样本（正好等于边界保留），返回当前来源状态。
func (s *Sync) evict(source string, nowMs int) *sourceState {
	st := s.state(source)
	cutoff := nowMs - s.windowMs
	dropped := 0
	for dropped < len(st.samples) && st.samples[dropped].atMs < cutoff {
		dropped++
	}
	if dropped > 0 {
		st.samples = st.samples[dropped:]
		s.scanned += dropped
		st.cacheValid = false
	}
	return st
}

// Offset 返回该来源窗口内的偏移中位数（偶数取中间偏左）与平均 RTT（算术平均）。
// 窗口内样本少于 minSamples 返回 ErrNoSamples。结果按窗口内样本集缓存，
// 只有回收或新增样本导致集合变化时才重新扫描。
func (s *Sync) Offset(source string, nowMs int) (int, int, error) {
	st := s.evict(source, nowMs)
	if len(st.samples) < s.minSamples {
		return 0, 0, ErrNoSamples{}
	}
	if st.cacheValid {
		return st.cacheOffset, st.cacheRtt, nil
	}
	offsets := make([]int, len(st.samples))
	rttTotal := 0
	for index, item := range st.samples {
		offsets[index] = item.offset
		rttTotal += item.rtt
	}
	sort.Ints(offsets)
	s.scanned += len(st.samples)
	st.cacheOffset = offsets[(len(offsets)-1)/2]
	st.cacheRtt = rttTotal / len(st.samples)
	st.cacheValid = true
	return st.cacheOffset, st.cacheRtt, nil
}

// Tick 在 nowMs 时刻下发一次校正，返回本次实际拨动的毫秒数与是否下发。
// 同一 (来源, 时刻) 只下发一次；超过 maxJump 的突变拦截并计入 Blocked；
// 拨动量为负按 0 处理；超出整秒预算的部分截断并计入 Deferred。
func (s *Sync) Tick(source string, nowMs int) (int, bool) {
	st := s.state(source)
	if st.ticks[nowMs] {
		return 0, false
	}
	estimate, _, err := s.Offset(source, nowMs)
	if err != nil {
		return 0, false
	}
	st.ticks[nowMs] = true

	diff := estimate - st.applied
	if diff > s.maxJump {
		s.Blocked++
		return 0, false
	}
	delta := diff
	if delta < 0 {
		delta = 0
	}

	bucket := nowMs / 1000
	if bucket != st.bucket {
		st.bucket = bucket
		st.spent = 0
	}
	if remaining := s.budgetMs - st.spent; delta > remaining {
		s.Deferred += delta - remaining
		delta = remaining
	}
	st.spent += delta
	st.applied += delta
	return delta, true
}

// DropSamples 清掉某来源的样本，用于模拟网络切换后的新窗口。
func (s *Sync) DropSamples(source string) {
	st := s.state(source)
	st.samples = nil
	st.cacheValid = false
}

// Sorted 返回排序副本，供调用方做中位数之类的参考计算。
func Sorted(values []int) []int {
	out := append([]int{}, values...)
	sort.Ints(out)
	return out
}

// Scanned 是累计扫描的样本数（规模观测）。
func (s *Sync) Scanned() int { return s.scanned }
