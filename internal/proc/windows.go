//go:build windows

package proc

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot lists processes via PowerShell so the tool stays dependency-free.
func Snapshot() ([]Process, error) {
	script := `Get-CimInstance Win32_Process | ForEach-Object { '{0}|{1}|{2}' -f $_.ProcessId, $_.ParentProcessId, $_.WorkingSetSize }`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	var procs []Process
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		pid, err1 := strconv.Atoi(parts[0])
		ppid, err2 := strconv.Atoi(parts[1])
		rss, err3 := strconv.ParseUint(parts[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		procs = append(procs, Process{PID: pid, PPID: ppid, RSS: rss})
	}
	return procs, nil
}

// KillTree stops the process and its children.
func KillTree(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	out, err := cmd.CombinedOutput()
	if err == nil || processGone(out) {
		return nil
	}
	return fmt.Errorf("taskkill pid %d: %w (%s)", pid, err, bytes.TrimSpace(out))
}

func processGone(out []byte) bool {
	s := strings.ToLower(string(out))
	return strings.Contains(s, "not found") ||
		strings.Contains(s, "no running instance") ||
		strings.Contains(s, "no such process")
}
