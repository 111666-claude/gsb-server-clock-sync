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

// sourceState 是单个来源的窗口台账：byTime 按采样时刻升序用于回收，
// offsets 按偏移升序维护多重集用于取中位数，rttSum 维护窗口内 RTT 之和。
type sourceState struct {
	byTime  []sample
	offsets []int
	rttSum  int
}

type tickKey struct {
	source string
	atMs   int
}

type secondKey struct {
	source string
	second int
}

// Sync 是时钟校正台账。
type Sync struct {
	windowMs   int
	minSamples int
	maxJump    int
	budgetMs   int
	sources    map[string]*sourceState
	applied    map[string]int
	spent      map[secondKey]int
	ticks      map[tickKey]bool
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
		applied: map[string]int{},
		spent:   map[secondKey]int{},
		ticks:   map[tickKey]bool{},
	}
}

// Add 记录一次四步握手：t1、t2 是本地发出与服务器收到，t3、t4 是服务器发出与本地收到。
// 只记录不估计；样本按时刻和偏移分别插入有序位置。
func (s *Sync) Add(source string, t1, t2, t3, t4, atMs int) {
	st := s.sources[source]
	if st == nil {
		st = &sourceState{}
		s.sources[source] = st
	}
	item := sample{
		offset: ((t2 - t1) + (t3 - t4)) / 2,
		rtt:    (t4 - t1) - (t3 - t2),
		atMs:   atMs,
	}
	timePos := sort.Search(len(st.byTime), func(i int) bool {
		return st.byTime[i].atMs > atMs
	})
	st.byTime = insertSample(st.byTime, timePos, item)
	offsetPos := sort.SearchInts(st.offsets, item.offset)
	st.offsets = insertInt(st.offsets, offsetPos, item.offset)
	st.rttSum += item.rtt
}

func insertSample(slice []sample, pos int, value sample) []sample {
	slice = append(slice, sample{})
	copy(slice[pos+1:], slice[pos:])
	slice[pos] = value
	return slice
}

func insertInt(slice []int, pos, value int) []int {
	slice = append(slice, 0)
	copy(slice[pos+1:], slice[pos:])
	slice[pos] = value
	return slice
}

// evict 回收距 nowMs 超过 windowMs 的样本（正好等于边界保留）。
// 每个样本至多被回收一次，所以累计扫描量不随估计调用次数增长。
func (s *Sync) evict(st *sourceState, nowMs int) {
	cut := 0
	for cut < len(st.byTime) && nowMs-st.byTime[cut].atMs > s.windowMs {
		cut++
	}
	if cut == 0 {
		return
	}
	s.scanned += cut
	for _, item := range st.byTime[:cut] {
		offsetPos := sort.SearchInts(st.offsets, item.offset)
		st.offsets = append(st.offsets[:offsetPos], st.offsets[offsetPos+1:]...)
		st.rttSum -= item.rtt
	}
	st.byTime = append(st.byTime[:0], st.byTime[cut:]...)
}

// Offset 返回该来源的偏移中位数（偶数个取中间偏左）与窗口内平均 RTT；
// 窗口内样本少于 minSamples 返回 ErrNoSamples。
func (s *Sync) Offset(source string, nowMs int) (int, int, error) {
	st := s.sources[source]
	if st == nil {
		return 0, 0, ErrNoSamples{}
	}
	s.evict(st, nowMs)
	count := len(st.byTime)
	if count == 0 || count < s.minSamples {
		return 0, 0, ErrNoSamples{}
	}
	return st.offsets[(count-1)/2], st.rttSum / count, nil
}

// Tick 下发一次校正，返回本次实际拨动的毫秒数。
// 同一 (来源, 时刻) 只下发一次，重复调用返回 (0, false)；
// 与已下发值之差超过 maxJump 时不下发并计入 Blocked；
// 差为负按 0 处理；同一整秒累计拨动不超过 budgetMs，放不下的部分计入 Deferred。
func (s *Sync) Tick(source string, nowMs int) (int, bool) {
	key := tickKey{source: source, atMs: nowMs}
	if s.ticks[key] {
		return 0, false
	}
	offset, _, err := s.Offset(source, nowMs)
	if err != nil {
		return 0, false
	}
	diff := offset - s.applied[source]
	if absInt(diff) > s.maxJump {
		s.Blocked++
		return 0, false
	}
	delta := diff
	if delta < 0 {
		delta = 0
	}
	budget := secondKey{source: source, second: nowMs / 1000}
	room := s.budgetMs - s.spent[budget]
	if delta > room {
		s.Deferred += delta - room
		delta = room
	}
	s.spent[budget] += delta
	s.applied[source] += delta
	s.ticks[key] = true
	return delta, true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// DropSamples 清掉某来源的样本，用于模拟网络切换后的新窗口；已下发值保留。
func (s *Sync) DropSamples(source string) {
	delete(s.sources, source)
}

// Sorted 返回排序副本，供调用方做中位数之类的参考计算。
func Sorted(values []int) []int {
	out := append([]int{}, values...)
	sort.Ints(out)
	return out
}

// Scanned 是累计扫描的样本数（规模观测）。
func (s *Sync) Scanned() int { return s.scanned }
