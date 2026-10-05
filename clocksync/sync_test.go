package clocksync

import "testing"

func TestNoSamplesIsError(t *testing.T) {
	if _, _, err := New(1000, 1, 500, 100).Offset("s1", 0); err == nil {
		t.Fatal("没有样本时应该报错")
	}
}

func TestSingleSampleOffset(t *testing.T) {
	book := New(1000, 1, 500, 100)
	book.Add("s1", 0, 10, 20, 50, 0)
	offset, rtt, err := book.Offset("s1", 0)
	if err != nil || offset != -10 || rtt != 40 {
		t.Fatalf("单样本应该给出 offset=-10 rtt=40，得到 %d %d %v", offset, rtt, err)
	}
}

func TestTickReturnsAppliedDelta(t *testing.T) {
	book := New(1000, 1, 500, 100)
	book.Add("s1", 0, 40, 40, 0, 0)
	delta, ok := book.Tick("s1", 0)
	if !ok || delta != 40 {
		t.Fatalf("第一次校正应该拨 40 毫秒，得到 %d ok=%v", delta, ok)
	}
}

func TestScannedStartsAtZero(t *testing.T) {
	if New(1000, 1, 500, 100).Scanned() != 0 {
		t.Fatal("Scanned 初始应该是 0")
	}
}
