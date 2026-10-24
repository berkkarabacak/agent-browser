package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareUserDataPreservesFieldsAndDropsSecurePrefs(t *testing.T) {
	dir := t.TempDir()
	prefs := filepath.Join(dir, "Default", "Preferences")
	mustWrite(t, prefs, []byte(`{"profile":{"name":"golden","exit_type":"Normal","exited_cleanly":true},"homepage":"https://example.com/?a=1&b=2"}`))
	mustWrite(t, filepath.Join(dir, "Default", "Secure Preferences"), []byte(`{"signed":true}`))
	mustWrite(t, filepath.Join(dir, "Local State"), []byte(`{"profile":{"info_cache":{"Default":{"name":"golden"}}}}`))

	if err := PrepareUserData(dir, "agent-001"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(prefs)
	if err != nil {
		t.Fatal(err)
	}
	if !bytesContains(body, "https://example.com/?a=1&b=2") {
		t.Fatalf("homepage was escaped or dropped: %s", body)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	profile := m["profile"].(map[string]any)
	if profile["name"] != "agent-001" {
		t.Fatalf("name = %#v", profile["name"])
	}
	if profile["exit_type"] != "Normal" {
		t.Fatalf("exit_type = %#v", profile["exit_type"])
	}
	if _, err := os.Stat(filepath.Join(dir, "Default", "Secure Preferences")); !os.IsNotExist(err) {
		t.Fatal("secure preferences should be removed when the name changes")
	}

	// A second pass with the same name must not require deleting a regenerated signature.
	mustWrite(t, filepath.Join(dir, "Default", "Secure Preferences"), []byte(`{"signed":true}`))
	if err := PrepareUserData(dir, "agent-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Default", "Secure Preferences")); err != nil {
		t.Fatal("secure preferences should stay when nothing changes")
	}
}

func bytesContains(b []byte, s string) bool {
	return len(b) >= len(s) && (string(b) == s || len(s) == 0 || contains(string(b), s))
}

func contains(hay, needle string) bool {
	return len(needle) == 0 || (len(hay) >= len(needle) && (hay == needle || len(hay) > 0 && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()))
}
