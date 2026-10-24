package profile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArgsExtensionsAndClass(t *testing.T) {
	dir := t.TempDir()
	ext := filepath.Join(dir, "ext")
	args, err := BuildArgs(ArgOptions{
		UserDataDir:    filepath.Join(dir, "user-data"),
		ExtensionPaths: []string{ext},
		Locale:         "en-US",
		Extra:          []string{"--disable-renderer-backgrounding"},
		ClassName:      "abrowser-agent-001",
		GOOS:           "linux",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	for _, want := range []string{
		"--user-data-dir=",
		"--load-extension=",
		"--disable-extensions-except=",
		"--lang=en-US",
		"--accept-lang=en-US",
		"--class=abrowser-agent-001",
		"--disable-renderer-backgrounding",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	var load, except string
	for _, a := range args {
		if strings.HasPrefix(a, "--load-extension=") {
			load = strings.TrimPrefix(a, "--load-extension=")
		}
		if strings.HasPrefix(a, "--disable-extensions-except=") {
			except = strings.TrimPrefix(a, "--disable-extensions-except=")
		}
	}
	if load == "" || load != except {
		t.Fatalf("load %q except %q", load, except)
	}
	if !strings.HasSuffix(args[len(args)-1], "--disable-renderer-backgrounding") {
		t.Fatalf("extra args should be last: %v", args)
	}
}

func TestBuildArgsOmitsClassOnWindows(t *testing.T) {
	dir := t.TempDir()
	args, err := BuildArgs(ArgOptions{
		UserDataDir: filepath.Join(dir, "user-data"),
		ClassName:   "abrowser-agent-001",
		GOOS:        "windows",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range args {
		if strings.Contains(a, "--class=") {
			t.Fatalf("windows args include class: %v", args)
		}
	}
}

func TestBuildArgsRejectsComma(t *testing.T) {
	dir := t.TempDir()
	_, err := BuildArgs(ArgOptions{
		UserDataDir:    filepath.Join(dir, "user-data"),
		ExtensionPaths: []string{filepath.Join(dir, "a,b")},
	})
	if err == nil {
		t.Fatal("expected comma error")
	}
}

func TestBuildEnvReplacesTZ(t *testing.T) {
	env := BuildEnv([]string{"PATH=/bin", "TZ=UTC", "HOME=/home/me"}, "America/New_York", "en-US")
	got := map[string]string{}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		got[k] = v
	}
	if got["TZ"] != "America/New_York" || got["PATH"] != "/bin" || got["HOME"] != "/home/me" {
		t.Fatalf("%v", env)
	}
	if got["LANGUAGE"] != "en-US" {
		t.Fatalf("LANGUAGE missing: %v", env)
	}
}
