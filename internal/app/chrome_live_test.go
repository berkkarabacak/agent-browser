//go:build integration

package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"abrowser/internal/layout"
	"abrowser/internal/profile"
)

// TestFiveLiveProfiles launches five real Chromium processes and checks
// isolation, per-profile extensions, and the double-launch lock.
func TestFiveLiveProfiles(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Setenv("DISPLAY", ":1")
	}
	browser := os.Getenv("ABROWSER_BROWSER")
	if browser == "" {
		browser = "/tmp/cft/chrome-linux64/chrome"
	}
	if st, err := os.Stat(browser); err != nil || st.IsDir() {
		t.Fatalf("browser %s: %v", browser, err)
	}

	root := t.TempDir()
	a, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetBrowser(browser); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetMaxConcurrent(5); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetExtraArgs([]string{
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-gpu",
		"--use-gl=angle",
		"--use-angle=swiftshader-webgl",
		"--password-store=basic",
		"--window-size=420,280",
		"--disable-background-networking",
	}); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(root, "src")
	commonSrc := filepath.Join(src, "common-marker")
	if err := writeLiveExt(commonSrc, "Common Marker", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddExtensionFromDir("common-marker", "1.0.0", commonSrc); err != nil {
		t.Fatal(err)
	}
	if err := a.CreateGroup("common"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GroupAdd("common", "common-marker"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("note-%03d", i)
		dir := filepath.Join(src, name)
		if err := writeLiveExt(dir, "Note "+fmt.Sprintf("%03d", i), "1.0.0"); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddExtensionFromDir(name, "1.0.0", dir); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.CreateTemplate(TemplateCreate{
		Name:     "golden",
		Timezone: "America/New_York",
		Locale:   "en-US",
		Groups:   []string{"common"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FreezeTemplate("golden"); err != nil {
		t.Fatal(err)
	}

	ids := make([]string, 0, 5)
	dirs := map[string]string{}
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("agent-%03d", i)
		ids = append(ids, id)
		if _, err := a.Clone(CloneRequest{
			Template:  "golden",
			ID:        id,
			Email:     id + "@example.com",
			Groups:    []string{"common"},
			GroupsSet: true,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.ProfileExt(id, "add", fmt.Sprintf("note-%03d", i)); err != nil {
			t.Fatal(err)
		}
		ud := layout.ProfileUserData(root, id)
		dirs[id] = ud
		t.Cleanup(func() { _ = a.CloseProfile(id) })
	}

	seenDir := map[string]string{}
	for id, dir := range dirs {
		if prev, ok := seenDir[dir]; ok {
			t.Fatalf("user-data dir shared by %s and %s: %s", prev, id, dir)
		}
		seenDir[dir] = id
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("user-data for %s: %v", id, err)
		}
	}

	launched := map[string]int{}
	for _, id := range ids {
		res, err := a.LaunchProfile(id, false)
		if err != nil {
			t.Fatalf("launch %s: %v\nlog:\n%s", id, err, readLog(layout.ProfileLog(root, id)))
		}
		if res.UserDataDir != dirs[id] {
			t.Fatalf("%s user-data %q want %q", id, res.UserDataDir, dirs[id])
		}
		launched[id] = res.PID
	}

	commonPath := layout.Extension(root, "common-marker", "1.0.0")
	for _, id := range ids {
		cmd, err := readCmdline(launched[id])
		if err != nil {
			t.Fatalf("%s pid %d cmdline: %v\nlog:\n%s", id, launched[id], err, readLog(layout.ProfileLog(root, id)))
		}
		if !strings.Contains(cmd, "--user-data-dir="+dirs[id]) {
			t.Fatalf("%s cmdline missing its user-data dir:\n%s", id, cmd)
		}
		for other, dir := range dirs {
			if other != id && strings.Contains(cmd, dir) {
				t.Fatalf("%s cmdline contains %s user-data:\n%s", id, other, cmd)
			}
		}
		if !strings.Contains(cmd, commonPath) {
			t.Fatalf("%s cmdline missing common-marker:\n%s", id, cmd)
		}
		own := layout.Extension(root, "note-"+id[len("agent-"):], "1.0.0")
		if !strings.Contains(cmd, own) {
			t.Fatalf("%s cmdline missing %s:\n%s", id, own, cmd)
		}
		for i := 1; i <= 5; i++ {
			note := fmt.Sprintf("note-%03d", i)
			path := layout.Extension(root, note, "1.0.0")
			want := note == "note-"+id[len("agent-"):]
			has := strings.Contains(cmd, path)
			if has != want {
				t.Fatalf("%s has %s = %v, want %v\n%s", id, note, has, want, cmd)
			}
		}
	}

	if _, err := a.LaunchProfile("agent-001", false); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("double launch err = %v", err)
	}
	st, err := a.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Running) != 5 {
		t.Fatalf("running %d, want 5 (max %d)", len(st.Running), st.MaxConcurrent)
	}
	if st.MaxConcurrent != 5 {
		t.Fatalf("max %d", st.MaxConcurrent)
	}
	still, err := readCmdline(launched["agent-001"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(still, "--user-data-dir="+dirs["agent-001"]) {
		t.Fatalf("double launch replaced agent-001:\n%s", still)
	}

	deadline := time.Now().Add(25 * time.Second)
	for _, id := range ids {
		own := "note-" + id[len("agent-"):] + "@1.0.0"
		for {
			prefs := profilePrefs(dirs[id])
			if filesContain(prefs, own) && filesContain(prefs, "common-marker@1.0.0") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s preferences never recorded %s and common-marker@1.0.0\nlog:\n%s", id, own, readLog(layout.ProfileLog(root, id)))
			}
			time.Sleep(500 * time.Millisecond)
		}
		prefs := profilePrefs(dirs[id])
		for i := 1; i <= 5; i++ {
			other := fmt.Sprintf("note-%03d@1.0.0", i)
			if other == own {
				continue
			}
			if filesContain(prefs, other) {
				t.Fatalf("%s preferences contain %s", id, other)
			}
		}
	}

	for _, id := range ids {
		if err := a.CloseProfile(id); err != nil {
			t.Fatal(err)
		}
	}
	st, err = a.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Running) != 0 {
		t.Fatalf("still running after close: %+v", st.Running)
	}

	page := filepath.Join(t.TempDir(), "iso.html")
	html := `<!doctype html><meta charset="utf-8"><title>isolation</title>
<script>
localStorage.setItem("abrowserIsolation", "only-agent-001");
document.cookie = "abrowserIsolation=only-agent-001; path=/";
</script><p>only-agent-001</p>`
	if err := os.WriteFile(page, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := profile.PrepareUserData(dirs["agent-001"], "agent-001"); err != nil {
		t.Fatal(err)
	}
	if err := runPage(browser, dirs["agent-001"], page); err != nil {
		t.Fatal(err)
	}
	if !treeContains(dirs["agent-001"], "only-agent-001") {
		t.Fatalf("agent-001 user-data has no localStorage/cookie marker after the page ran")
	}
	if treeContains(dirs["agent-002"], "only-agent-001") {
		t.Fatal("agent-002 user-data contains agent-001's marker")
	}
}

func writeLiveExt(dir, name, version string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("{\"manifest_version\":3,\"name\":%q,\"version\":%q,\"description\":\"abrowser live test\"}\n", name, version)
	return os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(body), 0o644)
}

func readCmdline(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(bytes.TrimRight(b, "\x00")), "\x00", " "), nil
}

func readLog(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	if len(b) > 4000 {
		b = b[len(b)-4000:]
	}
	return string(b)
}

func profilePrefs(userData string) []string {
	return []string{
		filepath.Join(userData, "Default", "Preferences"),
		filepath.Join(userData, "Default", "Secure Preferences"),
		filepath.Join(userData, "Local State"),
	}
}

func filesContain(paths []string, needle string) bool {
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if bytes.Contains(b, []byte(needle)) {
			return true
		}
	}
	return false
}

func treeContains(root, needle string) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Size() > 8<<20 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(b, []byte(needle)) {
			found = true
			return fmt.Errorf("stop")
		}
		return nil
	})
	return found
}

func runPage(browser, userData, page string) error {
	cmd := exec.Command(browser,
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-gpu",
		"--use-gl=angle",
		"--use-angle=swiftshader-webgl",
		"--password-store=basic",
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=400,240",
		"--user-data-dir="+userData,
		"file://"+page,
	)
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return err
	}
	time.Sleep(4 * time.Second)
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	if !treeContains(userData, "only-agent-001") {
		// One more graceful headless pass. Some builds flush localStorage
		// on a clean headless exit and not on SIGTERM.
		head := exec.Command(browser,
			"--headless=new",
			"--no-sandbox",
			"--disable-gpu",
			"--disable-dev-shm-usage",
			"--use-gl=angle",
			"--use-angle=swiftshader-webgl",
			"--password-store=basic",
			"--no-first-run",
			"--no-default-browser-check",
			"--user-data-dir="+userData,
			"--dump-dom",
			"file://"+page,
		)
		head.Env = os.Environ()
		out, err := head.CombinedOutput()
		if err != nil {
			return fmt.Errorf("headless isolation page: %w\n%s\nfirst run:\n%s", err, out, buf.String())
		}
	}
	return nil
}
