//go:build windows

package proc

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Alive reports whether pid is listed by tasklist.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), `"`+strconv.Itoa(pid)+`"`)
}
