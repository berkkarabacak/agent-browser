//go:build linux

package proc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Snapshot lists processes visible in /proc.
func Snapshot() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make([]Process, 0, len(entries))
	page := uint64(os.Getpagesize())
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil {
			continue
		}
		ppid, rssPages, err := readStat(pid)
		if err != nil {
			continue
		}
		out = append(out, Process{PID: pid, PPID: ppid, RSS: rssPages * page})
	}
	return out, nil
}

func readStat(pid int) (ppid int, rssPages uint64, err error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, err
	}
	s := string(b)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return 0, 0, fmt.Errorf("bad stat for %d", pid)
	}
	fields := strings.Fields(s[i+2:])
	// After comm: state, ppid, ... rss is field 24 of the full record,
	// which is index 21 in this slice.
	if len(fields) < 22 {
		return 0, 0, fmt.Errorf("short stat for %d", pid)
	}
	ppid, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, err
	}
	rssPages, err = strconv.ParseUint(fields[21], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return ppid, rssPages, nil
}
