package memory

import (
	"strings"
	"testing"
)

func TestSuggestMaxConcurrent(t *testing.T) {
	const gib = uint64(1 << 30)
	cases := []struct {
		total, free uint64
		want        int
	}{
		{16 * gib, 12 * gib, 8},
		{16 * gib, 3 * gib, 2},
		{32 * gib, 24 * gib, 16},
		{8 * gib, (15 * gib) / 2, 5},
		{16 * gib, 0, 5},
		{0, 0, 1},
		{18 * gib, 18 * gib, 8},
		{18*gib + 1, 30 * gib, 20},
	}
	for _, tc := range cases {
		got := SuggestMaxConcurrent(tc.total, tc.free)
		if got != tc.want {
			t.Fatalf("total=%d free=%d got %d want %d", tc.total, tc.free, got, tc.want)
		}
	}
}

func TestParseMeminfo(t *testing.T) {
	sample := "MemTotal:       16384000 kB\nMemFree:         1000000 kB\nMemAvailable:    8000000 kB\n"
	info, err := ParseMeminfo(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if info.Total != 16384000*1024 || info.Free != 8000000*1024 {
		t.Fatalf("%+v", info)
	}
	if _, err := SystemMemory(); err != nil {
		t.Fatal(err)
	}
}
