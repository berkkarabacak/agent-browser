//go:build unix

package proc

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// KillTree stops pid and its descendants, escalating to SIGKILL.
func KillTree(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := signalTree(pid, syscall.SIGTERM); err != nil {
		return err
	}
	if waitDead(pid, 2*time.Second) {
		return nil
	}
	if err := signalTree(pid, syscall.SIGKILL); err != nil {
		return err
	}
	if waitDead(pid, 2*time.Second) {
		return nil
	}
	return fmt.Errorf("process %d did not exit", pid)
}

func signalTree(pid int, sig syscall.Signal) error {
	snaps, err := Snapshot()
	if err != nil {
		return err
	}
	victims := append(Descendants(snaps, pid), pid)
	var first error
	for _, victim := range victims {
		err := syscall.Kill(victim, sig)
		if err != nil && !errors.Is(err, syscall.ESRCH) && first == nil {
			first = err
		}
	}
	return first
}
