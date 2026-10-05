package clocksync

import "testing"

func TestNoSamplesIsError(t *testing.T) {
	if _, _, err := New(1000, 1).Offset(0); err == nil {
		t.Fatal("没有样本时应该报错")
	}
}

func TestSingleSampleOffset(t *testing.T) {
	book := New(1000, 1)
	book.Add(0, 10, 20, 50, 0)
	offset, rtt, err := book.Offset(0)
	if err != nil || offset != -10 || rtt != 40 {
		t.Fatalf("单样本应该给出 offset=-10 rtt=40，得到 %d %d %v", offset, rtt, err)
	}
}

func TestScannedStartsAtZero(t *testing.T) {
	if New(1000, 1).Scanned() != 0 {
		t.Fatal("Scanned 初始应该是 0")
	}
}
