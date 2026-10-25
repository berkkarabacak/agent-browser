package webstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseID(t *testing.T) {
	const id = "eimadpbcbfnmbkopoojfekhnkhdbieeh"
	ok := []string{
		id,
		"  " + id + "  ",
		"https://chromewebstore.google.com/detail/dark-reader/" + id,
		"https://chromewebstore.google.com/detail/" + id,
		"https://chrome.google.com/webstore/detail/dark-reader/" + id + "?hl=en",
		"https://chromewebstore.google.com/detail/dark-reader/" + id + "?hl=en-US",
	}
	for _, in := range ok {
		got, err := ParseID(in)
		if err != nil || got != id {
			t.Fatalf("%q -> %q %v", in, got, err)
		}
	}
	for _, bad := range []string{
		"",
		"not-an-id",
		"eimadpbcbfnmbkopoojfekhnkhdbiee",
		"qimadpbcbfnmbkopoojfekhnkhdbieeh",
		"https://example.com/detail/" + id,
		"https://chromewebstore.google.com/detail/dark-reader",
	} {
		if _, err := ParseID(bad); err == nil {
			t.Fatalf("%q should fail", bad)
		}
	}
}

func TestExtensionSlug(t *testing.T) {
	const id = "eimadpbcbfnmbkopoojfekhnkhdbieeh"
	if got := ExtensionSlug("Dark Reader", id); got != "dark-reader" {
		t.Fatalf("got %q", got)
	}
	if got := ExtensionSlug("__MSG_appName__", id); got != "ext-"+id {
		t.Fatalf("got %q", got)
	}
	if got := ExtensionSlug("...", id); !strings.HasPrefix(got, "ext-") {
		t.Fatalf("got %q", got)
	}
}

func TestDownloadRejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>nope</html>"))
	}))
	defer srv.Close()
	old := CRXEndpoint
	CRXEndpoint = srv.URL
	t.Cleanup(func() { CRXEndpoint = old })
	dest := filepath.Join(t.TempDir(), "ext.crx")
	err := Download(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dest, srv.Client())
	if err == nil {
		t.Fatal("expected html to be rejected")
	}
}

func TestDownloadSavesCRX(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent")
		}
		_, _ = w.Write([]byte("Cr24payload"))
	}))
	defer srv.Close()
	old := CRXEndpoint
	CRXEndpoint = srv.URL
	t.Cleanup(func() { CRXEndpoint = old })
	dest := filepath.Join(t.TempDir(), "ext.crx")
	if err := Download(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dest, srv.Client()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "Cr24payload" {
		t.Fatalf("body %q", b)
	}
}
