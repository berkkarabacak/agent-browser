package proc

import "testing"

func TestDescendantsAndRSS(t *testing.T) {
	ps := []Process{
		{PID: 1, PPID: 0, RSS: 10},
		{PID: 2, PPID: 1, RSS: 20},
		{PID: 3, PPID: 2, RSS: 30},
		{PID: 4, PPID: 1, RSS: 40},
	}
	got := Descendants(ps, 1)
	want := []int{3, 2, 4}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if rss := TreeRSS(ps, 1); rss != 100 {
		t.Fatalf("rss %d", rss)
	}
	if rss := TreeRSS(ps, 99); rss != 0 {
		t.Fatalf("missing rss %d", rss)
	}
}
