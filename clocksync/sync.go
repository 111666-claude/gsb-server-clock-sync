// Package clocksync 估计服务器与本地时钟的偏移。
// 缺陷：采样窗口从不回收、偏移取平均而不是中位数、估计值会回退、样本不足也返回结果、每次估计全扫样本。
package clocksync

import "sort"

// ErrNoSamples 样本不足。
type ErrNoSamples struct{}

func (ErrNoSamples) Error() string { return "样本不足" }

type sample struct {
	rtt    int
	offset int
	atMs   int
}

// Sync 是时钟偏移台账。
type Sync struct {
	windowMs   int
	minSamples int
	samples    []sample
	lastOffset int
	scanned    int
}

// New 建台账：windowMs 是采样窗口，minSamples 是参与估计的最少样本数。
func New(windowMs, minSamples int) *Sync {
	return &Sync{windowMs: windowMs, minSamples: minSamples}
}

// Add 记录一次四步握手：t1、t2 是本地发出与服务器收到，t3、t4 是服务器发出与本地收到。
// 缺陷：不裁窗口、不查重复、全扫样本。
func (s *Sync) Add(t1, t2, t3, t4, atMs int) {
	s.samples = append(s.samples, sample{rtt: (t4 - t1) - (t3 - t2), offset: ((t2 - t1) + (t3 - t4)) / 2, atMs: atMs})
}

// Offset 返回偏移估计与平均 RTT。缺陷：窗口不滑、用平均、不单调、样本不足也返回、全扫样本。
func (s *Sync) Offset(nowMs int) (int, int, error) {
	s.scanned += len(s.samples)
	if len(s.samples) == 0 {
		return 0, 0, ErrNoSamples{}
	}
	values := []int{}
	total := 0
	for _, item := range s.samples {
		values = append(values, item.offset)
		total += item.rtt
	}
	sort.Ints(values)
	offset := 0
	for _, value := range values {
		offset += value
	}
	offset /= len(values)
	s.lastOffset = offset
	return offset, total / len(values), nil
}

// Scanned 是累计扫描的样本数（规模观测）。
func (s *Sync) Scanned() int { return s.scanned }
