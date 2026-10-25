package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"abrowser/internal/memory"
	"abrowser/internal/webstore"
)

func TestPrepareAndCreatePerson(t *testing.T) {
	root := t.TempDir()
	browser := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(browser, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABROWSER_BROWSER", browser)

	svc, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	info, err := memory.SystemMemory()
	if err != nil {
		t.Fatal(err)
	}
	wantMax := memory.SuggestMaxConcurrent(info.Total, info.Free)
	first, err := svc.PrepareFirstRun(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.MaxConcurrent != wantMax {
		t.Fatalf("max %d want %d", first.MaxConcurrent, wantMax)
	}
	if first.Downloaded {
		t.Fatal("preset browser should skip the download")
	}
	if first.BrowserPath != browser {
		t.Fatalf("browser %q", first.BrowserPath)
	}

	mom, err := svc.CreatePerson("Mom", "mom@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if mom.Profile.ID != "mom" || mom.Profile.DisplayName != "Mom" {
		t.Fatalf("mom: %+v", mom.Profile)
	}
	foundCommon := false
	for _, g := range mom.Profile.Groups {
		if g == "common" {
			foundCommon = true
		}
	}
	if !foundCommon {
		t.Fatalf("groups %v", mom.Profile.Groups)
	}
	again, err := svc.CreatePerson("Mom", "mom2@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if again.Profile.ID != "mom-2" || again.Profile.DisplayName != "Mom" {
		t.Fatalf("second: %+v", again.Profile)
	}

	snap, err := svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Lines) != 2 || snap.Lines[0].Title() == "" {
		t.Fatalf("lines %+v", snap.Lines)
	}
	frozen := false
	for _, tmpl := range snap.Templates {
		if tmpl.Name == "golden" && tmpl.Frozen {
			frozen = true
		}
	}
	if !frozen {
		t.Fatalf("templates %+v", snap.Templates)
	}

	payload := crx2(t, `{
	  "manifest_version": 3,
	  "name": "Dark Reader",
	  "version": "1.0.0"
	}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	old := webstore.CRXEndpoint
	webstore.CRXEndpoint = srv.URL
	t.Cleanup(func() { webstore.CRXEndpoint = old })

	ext, err := svc.AddWebStore(context.Background(), "https://chromewebstore.google.com/detail/dark-reader/eimadpbcbfnmbkopoojfekhnkhdbieeh", "common")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Ref() != "dark-reader@1.0.0" {
		t.Fatalf("ext %+v", ext)
	}
	ext, err = svc.AddWebStore(context.Background(), "eimadpbcbfnmbkopoojfekhnkhdbieeh", "mom")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Ref() != "dark-reader@1.0.0" {
		t.Fatalf("second ext %+v", ext)
	}
	view, err := svc.Show("mom")
	if err != nil {
		t.Fatal(err)
	}
	pinned := false
	for _, ref := range view.Profile.AddExtensions {
		if ref == "dark-reader@1.0.0" {
			pinned = true
		}
	}
	if !pinned {
		t.Fatalf("overrides %+v groups %v", view.Profile.AddExtensions, view.Profile.Groups)
	}
	if err := svc.FinishSetup(); err != nil {
		t.Fatal(err)
	}
	snap, err = svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Config.SetupComplete {
		t.Fatal("setup not complete")
	}
	if _, err := svc.CreatePerson("...", "bad"); err == nil {
		t.Fatal("expected a rejected name")
	}
}

func crx2(t *testing.T, manifest string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
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
	payload := buf.Bytes()
	pub := []byte("pub")
	sig := []byte("sig")
	var out bytes.Buffer
	out.WriteString("Cr24")
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(pub)))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(sig)))
	out.Write(pub)
	out.Write(sig)
	out.Write(payload)
	return out.Bytes()
}
