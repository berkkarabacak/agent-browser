package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBadgeText(t *testing.T) {
	if got := BadgeText("agent-001"); got != "T001" {
		t.Fatalf("got %s", got)
	}
	if got := BadgeText("ab"); got != "AB" {
		t.Fatalf("got %s", got)
	}
	if got := BadgeText("---"); got != "AB" {
		t.Fatalf("got %s", got)
	}
}

func TestWriteBadgeEscapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "badge")
	if err := WriteBadge(dir, "agent-001", `ada+a@example.com`); err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile(filepath.Join(dir, "sw.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"agent-001"`) || !strings.Contains(string(js), "T001") {
		t.Fatalf("worker:\n%s", js)
	}
}
