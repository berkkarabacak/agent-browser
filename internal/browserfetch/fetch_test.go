package browserfetch

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchUnpacksChrome(t *testing.T) {
	payload := zipBytes(t, map[string]string{
		"chrome-linux64/chrome": "#!/bin/sh\necho chrome\n",
	})
	var sawProgress bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/versions.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"channels":{"Stable":{"version":"156.0.1","downloads":{"chrome":[{"platform":"linux64","url":"` + "http://" + r.Host + `/chrome.zip"}]}}}}`))
			return
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	dir := t.TempDir()
	res, err := Fetch(context.Background(), Options{
		OS:          "linux",
		Arch:        "amd64",
		DestDir:     dir,
		Client:      srv.Client(),
		ManifestURL: srv.URL + "/versions.json",
		Progress: func(done, total int64) {
			if done > 0 {
				sawProgress = true
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "156.0.1" {
		t.Fatalf("version %q", res.Version)
	}
	want := filepath.Join(dir, "chrome-linux64", "chrome")
	if res.Executable != want {
		t.Fatalf("exe %q", res.Executable)
	}
	st, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatalf("chrome is not executable: %v", st.Mode())
	}
	if !sawProgress {
		t.Fatal("progress was not reported")
	}
}

func TestUnzipRejectsSlip(t *testing.T) {
	payload := zipBytes(t, map[string]string{
		"../evil":               "nope",
		"chrome-linux64/chrome": "ok",
	})
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(zipPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := Unzip(zipPath, dest); err == nil {
		t.Fatal("expected zip slip to fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Fatal("zip slip wrote evil next to the destination")
	}
}

func TestPlatform(t *testing.T) {
	key, exe, err := Platform("windows", "amd64")
	if err != nil || key != "win64" || exe != "chrome-win64/chrome.exe" {
		t.Fatalf("%s %s %v", key, exe, err)
	}
	if _, _, err := Platform("windows", "386"); err == nil {
		t.Fatal("expected unsupported platform")
	}
}
