package cli

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"abrowser/internal/proc"
)

func TestMain(m *testing.M) {
	// The end-to-end test re-executes this binary as a stand-in Chromium.
	// A real `go test` process always has -test.* arguments, so the variable
	// alone does not swallow a normal test run.
	if os.Getenv("ABROWSER_TEST_FAKE_BROWSER") == "1" && !hasTestFlag(os.Args[1:]) {
		fakeBrowser()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func hasTestFlag(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

func fakeBrowser() {
	tz := os.Getenv("TZ")
	var dir string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--user-data-dir=") {
			dir = strings.TrimPrefix(arg, "--user-data-dir=")
		}
	}
	if dir != "" {
		parent := filepath.Dir(dir)
		_ = os.WriteFile(filepath.Join(parent, "launched-args.txt"), []byte(strings.Join(os.Args[1:], "\n")+"\n"), 0o644)
		_ = os.WriteFile(filepath.Join(parent, "launched-tz.txt"), []byte(tz), 0o644)
	}
	time.Sleep(3 * time.Minute)
}

func TestEndToEnd(t *testing.T) {
	t.Setenv("ABROWSER_TEST_FAKE_BROWSER", "1")
	root := t.TempDir()
	cleanupBrowsers(t, root)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	hello := writeUnpacked(t, "Hello", "1.4.0")
	extra := writeUnpacked(t, "Extra", "0.3.0")
	crx := writeCRX(t, "Sales Helper", "2.1.0")

	mustRun(t, root, "init")
	mustRun(t, root, "config", "set", "browser", exe)
	mustRun(t, root, "config", "set", "max-concurrent", "20")
	mustRun(t, root, "ext", "add", "hello", "--from", hello)
	mustRun(t, root, "ext", "add", "sales-helper", "--crx", crx)
	mustRun(t, root, "ext", "add", "extra", "--from", extra)
	extList := mustRun(t, root, "ext", "list")
	for _, want := range []string{"hello", "1.4.0", "sales-helper", "2.1.0", "extra", "0.3.0"} {
		if !strings.Contains(extList, want) {
			t.Fatalf("ext list missing %s:\n%s", want, extList)
		}
	}

	mustRun(t, root, "group", "create", "common")
	mustRun(t, root, "group", "add", "common", "hello")
	mustRun(t, root, "group", "create", "sales")
	pinned := mustRun(t, root, "group", "add", "sales", "sales-helper")
	if !strings.Contains(pinned, "sales-helper@2.1.0") {
		t.Fatalf("group add did not pin version:\n%s", pinned)
	}

	mustRun(t, root, "template", "create", "golden", "--timezone", "America/New_York", "--locale", "en-US", "--group", "common")
	if out := mustFail(t, root, "clone", "golden", "--as", "too-soon", "--email", "ada@example.com"); !strings.Contains(out, "not frozen") {
		t.Fatalf("clone before freeze: %s", out)
	}
	seedTemplate(t, root)
	mustRun(t, root, "template", "launch", "golden")
	tz := waitFile(t, filepath.Join(root, "templates", "golden", "launched-tz.txt"))
	if strings.TrimSpace(string(tz)) != "America/New_York" {
		t.Fatalf("template TZ %q", tz)
	}
	args := string(waitFile(t, filepath.Join(root, "templates", "golden", "launched-args.txt")))
	if !strings.Contains(args, "--load-extension=") || !strings.Contains(args, "hello@1.4.0") || !strings.Contains(args, "--disable-extensions-except=") {
		t.Fatalf("template args:\n%s", args)
	}
	mustRun(t, root, "template", "close", "golden")
	frozen := mustRun(t, root, "template", "freeze", "golden")
	if !strings.Contains(frozen, "Froze template golden") {
		t.Fatalf("freeze output:\n%s", frozen)
	}
	if out := mustFail(t, root, "template", "launch", "golden"); !strings.Contains(out, "frozen") {
		t.Fatalf("launch frozen: %s", out)
	}

	mustRun(t, root, "clone", "golden", "--as", "agent-001", "--email", "ada@example.com", "--group", "common", "--group", "sales", "--notes", "east coast")
	mustRun(t, root, "clone", "golden", "--as", "agent-002", "--email", "grace@example.com")
	mustRun(t, root, "clone", "golden", "--as", "agent-003", "--email", "lin@example.com", "--group", "common", "--group", "sales", "--timezone", "Europe/Berlin")

	assertCloneSlim(t, root)
	show := mustProfile(t, root, "agent-001")
	if show.Profile.Email != "ada@example.com" || show.Profile.Notes != "east coast" {
		t.Fatalf("profile %+v", show.Profile)
	}
	if strings.Join(show.Profile.Groups, ",") != "common,sales" {
		t.Fatalf("groups %v", show.Profile.Groups)
	}
	if refs := resolvedRefs(show); strings.Join(refs, ",") != "hello@1.4.0,sales-helper@2.1.0" {
		t.Fatalf("resolved %v", refs)
	}
	show2 := mustProfile(t, root, "agent-002")
	if strings.Join(show2.Profile.Groups, ",") != "common" {
		t.Fatalf("inherited groups %v", show2.Profile.Groups)
	}
	if refs := resolvedRefs(show2); strings.Join(refs, ",") != "hello@1.4.0" {
		t.Fatalf("agent-002 resolved %v", refs)
	}

	dry := mustRun(t, root, "launch", "agent-001", "--dry-run")
	if !strings.Contains(dry, "--user-data-dir=") || !strings.Contains(dry, "hello@1.4.0") || !strings.Contains(dry, "TZ=America/New_York") {
		t.Fatalf("dry run:\n%s", dry)
	}

	mustRun(t, root, "launch", "agent-001")
	mustRun(t, root, "launch", "agent-002")
	mustRun(t, root, "launch", "agent-003")
	if got := strings.TrimSpace(string(waitFile(t, filepath.Join(root, "profiles", "agent-003", "launched-tz.txt")))); got != "Europe/Berlin" {
		t.Fatalf("agent-003 TZ %q", got)
	}
	status := mustRun(t, root, "status")
	for _, id := range []string{"agent-001", "agent-002", "agent-003"} {
		if !strings.Contains(status, id) {
			t.Fatalf("status missing %s:\n%s", id, status)
		}
	}
	if !strings.Contains(status, "approx RAM") {
		t.Fatalf("status missing RAM:\n%s", status)
	}
	if out := mustFail(t, root, "launch", "agent-001"); !strings.Contains(out, "already running") {
		t.Fatalf("double launch: %s", out)
	}
	mustRun(t, root, "close", "agent-001")
	mustRun(t, root, "close", "agent-002")
	mustRun(t, root, "close", "agent-003")

	mustRun(t, root, "config", "set", "max-concurrent", "2")
	mustRun(t, root, "config", "set", "auto-close-lru", "false")
	mustRun(t, root, "launch", "agent-001")
	mustRun(t, root, "launch", "agent-002")
	if out := mustFail(t, root, "launch", "agent-003"); !strings.Contains(out, "profile limit") {
		t.Fatalf("limit: %s", out)
	}
	pid := readPID(t, filepath.Join(root, "profiles", "agent-001", "running.json"))
	mustRun(t, root, "config", "set", "auto-close-lru", "true")
	mustRun(t, root, "launch", "agent-003")
	if proc.Alive(pid) {
		t.Fatal("least recently launched profile was not closed")
	}
	if _, err := os.Stat(filepath.Join(root, "profiles", "agent-001", "running.json")); !os.IsNotExist(err) {
		t.Fatalf("agent-001 still recorded as running: %v", err)
	}
	mustRun(t, root, "close", "agent-002")
	mustRun(t, root, "close", "agent-003")

	stale := filepath.Join(root, "profiles", "agent-002", "running.json")
	if err := os.WriteFile(stale, []byte("{\"kind\":\"profile\",\"id\":\"agent-002\",\"pid\":2147483000}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, root, "launch", "agent-002")
	mustRun(t, root, "close", "agent-002")

	mustRun(t, root, "profile", "ext", "add", "agent-002", "extra")
	mustRun(t, root, "profile", "ext", "remove", "agent-002", "hello")
	over := mustProfile(t, root, "agent-002")
	if refs := resolvedRefs(over); strings.Join(refs, ",") != "extra@0.3.0" {
		t.Fatalf("overrides resolved %v (add %v remove %v)", refs, over.Profile.AddExtensions, over.Profile.RemoveExtensions)
	}

	if out := mustFail(t, root, "delete", "agent-003"); !strings.Contains(out, "--yes") {
		t.Fatalf("delete confirmation: %s", out)
	}
	mustRun(t, root, "delete", "agent-003", "--yes")
	listed := mustRun(t, root, "list")
	if strings.Contains(listed, "agent-003") {
		t.Fatalf("deleted profile still listed:\n%s", listed)
	}
	if !strings.Contains(listed, "agent-001") || !strings.Contains(listed, "agent-002") {
		t.Fatalf("list:\n%s", listed)
	}
}

type profileJSON struct {
	Profile struct {
		ID               string   `json:"id"`
		Email            string   `json:"email"`
		Notes            string   `json:"notes"`
		Groups           []string `json:"groups"`
		AddExtensions    []string `json:"addExtensions"`
		RemoveExtensions []string `json:"removeExtensions"`
	} `json:"profile"`
	Resolved []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"resolved"`
}

func mustProfile(t *testing.T, root, id string) profileJSON {
	t.Helper()
	out := mustRun(t, root, "profile", "show", id, "--json")
	var view profileJSON
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("profile json: %v\n%s", err, out)
	}
	return view
}

func resolvedRefs(view profileJSON) []string {
	refs := make([]string, 0, len(view.Resolved))
	for _, ext := range view.Resolved {
		refs = append(refs, ext.Name+"@"+ext.Version)
	}
	return refs
}

func seedTemplate(t *testing.T, root string) {
	t.Helper()
	base := filepath.Join(root, "templates", "golden", "user-data")
	prefs := `{
  "profile": {"name": "golden", "avatar_index": 1, "exit_type": "Normal", "exited_cleanly": true},
  "homepage": "https://example.com/?a=1&b=2"
}`
	writeFile(t, filepath.Join(base, "Default", "Preferences"), prefs)
	writeFile(t, filepath.Join(base, "Local State"), `{"profile":{"info_cache":{"Default":{"name":"golden"}}}}`)
	writeFile(t, filepath.Join(base, "Default", "Cache", "junk.bin"), "cache-bytes")
	writeFile(t, filepath.Join(base, "Default", "Local Storage", "leveldb", "CURRENT"), "keep-me")
	writeFile(t, filepath.Join(base, "Default", "Service Worker", "CacheStorage", "junk"), "sw-cache")
	writeFile(t, filepath.Join(base, "Default", "Service Worker", "Database", "CURRENT"), "sw-db")
	writeFile(t, filepath.Join(base, "ShaderCache", "data"), "shader")
	writeFile(t, filepath.Join(base, "SingletonLock"), "lock")
	if err := os.Symlink("ignored", filepath.Join(base, "Default", "SingletonCookie")); err != nil {
		t.Fatal(err)
	}
}

func assertCloneSlim(t *testing.T, root string) {
	t.Helper()
	base := filepath.Join(root, "profiles", "agent-001", "user-data")
	mustExist(t, filepath.Join(base, "Default", "Local Storage", "leveldb", "CURRENT"))
	mustExist(t, filepath.Join(base, "Default", "Service Worker", "Database", "CURRENT"))
	mustAbsent(t, filepath.Join(base, "Default", "Cache"))
	mustAbsent(t, filepath.Join(base, "Default", "Service Worker", "CacheStorage"))
	mustAbsent(t, filepath.Join(base, "ShaderCache"))
	mustAbsent(t, filepath.Join(base, "SingletonLock"))
	mustAbsent(t, filepath.Join(base, "Default", "SingletonCookie"))
	body, err := os.ReadFile(filepath.Join(base, "Default", "Preferences"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "https://example.com/?a=1&b=2") {
		t.Fatalf("homepage lost: %s", body)
	}
	var prefs map[string]any
	if err := json.Unmarshal(body, &prefs); err != nil {
		t.Fatal(err)
	}
	profile := prefs["profile"].(map[string]any)
	if profile["name"] != "agent-001" {
		t.Fatalf("profile name %#v", profile["name"])
	}
}

func writeUnpacked(t *testing.T, manifestName, version string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ext")
	body := `{
  "manifest_version": 3,
  "name": "` + manifestName + `",
  "version": "` + version + `"
}`
	writeFile(t, filepath.Join(dir, "manifest.json"), body)
	return dir
}

func writeCRX(t *testing.T, manifestName, version string) string {
	t.Helper()
	manifest := `{
  "manifest_version": 3,
  "name": "` + manifestName + `",
  "version": "` + version + `"
}`
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	payload := zipBuf.Bytes()
	var out bytes.Buffer
	out.WriteString("Cr24")
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))
	pub, sig := []byte("pub"), []byte("sig")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(pub)))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(sig)))
	out.Write(pub)
	out.Write(sig)
	out.Write(payload)
	path := filepath.Join(t.TempDir(), "ext.crx")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func cleanupBrowsers(t *testing.T, root string) {
	t.Helper()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != "running.json" {
				return nil
			}
			var st struct {
				PID int `json:"pid"`
			}
			b, err := os.ReadFile(path)
			if err == nil {
				_ = json.Unmarshal(b, &st)
			}
			if st.PID > 0 {
				_ = proc.KillTree(st.PID)
			}
			return nil
		})
	})
}

func mustRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, errb, code := runCLI(root, args...)
	if code != 0 {
		t.Fatalf("abrowser %s\nexit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, out, errb)
	}
	return out
}

func mustFail(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, errb, code := runCLI(root, args...)
	if code == 0 {
		t.Fatalf("expected failure for %s\nstdout:\n%s", strings.Join(args, " "), out)
	}
	return errb + "\n" + out
}

func runCLI(root string, args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := Run(append([]string{"--root", root}, args...), IO{
		In:          bytes.NewReader(nil),
		Out:         &out,
		Err:         &errb,
		Interactive: false,
	})
	return out.String(), errb.String(), code
}

func waitFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil && len(bytes.TrimSpace(b)) > 0 {
			return b
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", path)
	return nil
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if st.PID <= 0 {
		t.Fatalf("pid in %s: %s", path, b)
	}
	return st.PID
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing %s: %v", path, err)
	}
}

func mustAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, err=%v", path, err)
	}
}
