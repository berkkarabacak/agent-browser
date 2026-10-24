package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// StartSpec launches a browser process detached from the CLI's controlling terminal.
type StartSpec struct {
	Browser string
	Args    []string
	Env     []string
	LogPath string
}

// StartBrowser starts the browser and returns the running command.
// The caller must Wait on it (a background goroutine is enough) so the
// process is reaped if it exits before this process does.
func StartBrowser(spec StartSpec) (*exec.Cmd, error) {
	browser := strings.TrimSpace(spec.Browser)
	if browser == "" {
		return nil, fmt.Errorf("browser path is empty")
	}
	if _, err := os.Stat(browser); err != nil {
		return nil, fmt.Errorf("browser not found at %s", browser)
	}
	if err := os.MkdirAll(filepath.Dir(spec.LogPath), 0o755); err != nil {
		return nil, err
	}
	logFile, err := os.Create(spec.LogPath)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(logFile, "[abrowser] %s\n[abrowser] argv:", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(logFile, " %s", strconv.Quote(browser))
	for _, arg := range spec.Args {
		fmt.Fprintf(logFile, " %s", strconv.Quote(arg))
	}
	fmt.Fprintln(logFile)
	cmd := exec.Command(browser, spec.Args...)
	cmd.Env = spec.Env
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	// The child inherited the log fd.
	_ = logFile.Close()
	return cmd, nil
}
