//go:build !linux && !windows

package proc

import (
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot uses ps for operating systems other than Linux and Windows.
func Snapshot() ([]Process, error) {
	out, err := exec.Command("ps", "-ax", "-o", "pid=,ppid=,rss=").Output()
	if err != nil {
		return nil, err
	}
	var procs []Process
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		rssKB, err3 := strconv.ParseUint(fields[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		procs = append(procs, Process{PID: pid, PPID: ppid, RSS: rssKB * 1024})
	}
	return procs, nil
}
