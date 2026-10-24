//go:build linux

package proc

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSnapshotSelf(t *testing.T) {
	snaps, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	self := os.Getpid()
	found := false
	for _, p := range snaps {
		if p.PID == self {
			found = true
			if p.PPID != os.Getppid() {
				t.Fatalf("ppid %d want %d", p.PPID, os.Getppid())
			}
			if p.RSS == 0 {
				t.Fatal("rss is 0")
			}
		}
	}
	if !found {
		t.Fatal("self not in snapshot")
	}
}

func TestKillTree(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	if err := KillTree(pid); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sleep still running")
	}
	if Alive(pid) {
		t.Fatal("pid still alive")
	}
}
