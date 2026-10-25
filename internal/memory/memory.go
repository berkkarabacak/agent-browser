// Package memory reads free RAM and suggests how many Chromes can stay open.
package memory

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Info is installed RAM and memory that is free for new profiles.
type Info struct {
	Total uint64
	Free  uint64
}

const (
	perProfile = (3 * (1 << 30)) / 2 // 1.5 GiB
	gib        = 1 << 30
)

// SuggestMaxConcurrent is about one running profile per 1.5 GiB free.
// A machine with 18 GiB or less is capped at 8, so a 16 GB PC stays at 8.
// Larger machines are capped at 20. The result is at least 1.
func SuggestMaxConcurrent(total, free uint64) int {
	if free == 0 && total > 0 {
		free = total / 2
	}
	n := 1
	if free > 0 {
		n = int(free / perProfile)
		if n < 1 {
			n = 1
		}
	}
	capN := 20
	if total > 0 && total <= 18*gib {
		capN = 8
	}
	if n > capN {
		n = capN
	}
	return n
}

// ParseMeminfo reads MemTotal and MemAvailable, in bytes, from a /proc/meminfo document.
func ParseMeminfo(r io.Reader) (Info, error) {
	var info Info
	var sawTotal, sawFree bool
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		var kb uint64
		if _, err := fmt.Sscanf(fields[1], "%d", &kb); err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			info.Total = kb * 1024
			sawTotal = true
		case "MemAvailable":
			info.Free = kb * 1024
			sawFree = true
		}
	}
	if err := sc.Err(); err != nil {
		return Info{}, err
	}
	if !sawTotal {
		return Info{}, fmt.Errorf("MemTotal missing from meminfo")
	}
	if !sawFree {
		return Info{}, fmt.Errorf("MemAvailable missing from meminfo")
	}
	return info, nil
}
