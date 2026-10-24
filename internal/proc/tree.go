// Package proc inspects and stops browser process trees.
package proc

import (
	"time"
)

// Process is one OS process.
type Process struct {
	PID  int
	PPID int
	RSS  uint64
}

// Descendants returns pids under root, leaves first, excluding root.
func Descendants(ps []Process, root int) []int {
	children := map[int][]int{}
	for _, p := range ps {
		children[p.PPID] = append(children[p.PPID], p.PID)
	}
	var out []int
	var walk func(int)
	walk = func(pid int) {
		for _, child := range children[pid] {
			walk(child)
			out = append(out, child)
		}
	}
	walk(root)
	return out
}

// TreeRSS sums the working set of root and its descendants.
// Shared pages are counted once per process, so this is an approximation.
func TreeRSS(ps []Process, root int) uint64 {
	byPID := map[int]Process{}
	for _, p := range ps {
		byPID[p.PID] = p
	}
	if _, ok := byPID[root]; !ok {
		return 0
	}
	total := byPID[root].RSS
	for _, pid := range Descendants(ps, root) {
		total += byPID[pid].RSS
	}
	return total
}

func waitDead(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if !Alive(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(30 * time.Millisecond)
	}
}
