package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"abrowser/internal/layout"
	"abrowser/internal/proc"
)

func (a *App) withLock(fn func() error) error {
	if err := os.MkdirAll(a.Root, 0o755); err != nil {
		return err
	}
	unlock, err := a.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func (a *App) lock() (func(), error) {
	path := layout.Lock(a.Root)
	deadline := time.Now().Add(60 * time.Second)
	emptyReads := 0
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if _, werr := fmt.Fprintf(f, "%d\n", os.Getpid()); werr != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, werr
			}
			return func() {
				_ = f.Close()
				if pid, ok := readLockPID(path); ok && pid == os.Getpid() {
					_ = os.Remove(path)
				}
			}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		pid, ok := readLockPID(path)
		if !ok || pid == 0 {
			emptyReads++
			if emptyReads > 20 {
				_ = os.Remove(path)
				emptyReads = 0
			}
		} else {
			emptyReads = 0
			if pid != os.Getpid() && !proc.Alive(pid) {
				_ = os.Remove(path)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func readLockPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return pid, true
}
