package profile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkip(t *testing.T) {
	keep := []string{
		"Default/Preferences",
		"Default/Local Storage/leveldb/CURRENT",
		"Default/IndexedDB/x",
		"Default/Extensions/id/manifest.json",
		"Default/Local Extension Settings/id/x",
		"Default/Local Extension Settings/abc/Cache/data",
		"Default/Cookies",
		"Local State",
		"Default/Bookmarks",
		"Default/Service Worker/Database/CURRENT",
		"Default/Session Storage/x",
		"Default/Network/Cookies",
		"Default/Sessions/Session_1",
		"Default/Current Session",
		"Default/Last Tabs",
	}
	skip := []string{
		"Default/Cache/a",
		"Default/Code Cache/js/a",
		"Default/GPUCache/a",
		"Default/Service Worker/CacheStorage/a",
		"Default/Service Worker/ScriptCache/a",
		"ShaderCache/a",
		"GrShaderCache/a",
		"GraphiteDawnCache/a",
		"Crashpad/a",
		"BrowserMetrics/a",
		"component_crx_cache/a",
		"extensions_crx_cache/a",
		"Safe Browsing/a",
		"Crowd Deny/a",
		"Default/blob_storage/a",
		"Default/File System/a",
		"Default/DawnGraphiteCache/a",
		"Default/DawnWebGPUCache/a",
		"SingletonLock",
		"SingletonCookie",
		"SingletonSocket",
		"lockfile",
		"Default/LOG",
		"optimization_guide_hint_cache_store/a",
		"OnDeviceHeadSuggestModel/a",
		"Default/Shared Dictionary/db",
		"Crash Reports/a",
		"Default/Network/Cache/data.bin",
	}
	for _, rel := range keep {
		mode := fs.FileMode(0o644)
		if err := shouldSkipPath(rel, mode); err {
			t.Errorf("kept path %s was skipped", rel)
		}
	}
	for _, rel := range skip {
		// Directory components are skipped when the walker sees the directory.
		// Assert the first matching ancestor is skipped, and the file itself
		// is skipped when it is a known junk file.
		if !pathSkipped(rel) {
			t.Errorf("path %s was kept", rel)
		}
	}
}

func shouldSkipPath(rel string, mode fs.FileMode) bool {
	return ShouldSkip(rel, mode)
}

func pathSkipped(rel string) bool {
	parts := splitSlash(rel)
	cur := ""
	for i, part := range parts {
		if cur == "" {
			cur = part
		} else {
			cur = cur + "/" + part
		}
		mode := fs.FileMode(0o644)
		if i < len(parts)-1 {
			mode = fs.ModeDir
		}
		if ShouldSkip(cur, mode) {
			return true
		}
	}
	return false
}

func splitSlash(rel string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(rel); i++ {
		if i == len(rel) || rel[i] == '/' {
			if i > start {
				out = append(out, rel[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func TestCopyUserDataOmitsCaches(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "Default", "Preferences"), []byte(`{"profile":{"name":"golden"},"homepage":"https://example.com/?a=1&b=2"}`))
	mustWrite(t, filepath.Join(src, "Default", "Cache", "junk.bin"), []byte("cache-bytes"))
	mustWrite(t, filepath.Join(src, "Default", "Code Cache", "js", "a"), []byte("code"))
	mustWrite(t, filepath.Join(src, "Default", "Local Storage", "leveldb", "CURRENT"), []byte("keep-me"))
	mustWrite(t, filepath.Join(src, "Default", "Service Worker", "CacheStorage", "junk"), []byte("sw-cache"))
	mustWrite(t, filepath.Join(src, "Default", "Service Worker", "Database", "CURRENT"), []byte("sw-db"))
	mustWrite(t, filepath.Join(src, "Default", "Local Extension Settings", "abc", "Cache", "data"), []byte("ext-cache-keep"))
	mustWrite(t, filepath.Join(src, "Default", "Network", "Cache", "data.bin"), []byte("net-cache"))
	mustWrite(t, filepath.Join(src, "Default", "Network", "Cookies"), []byte("cookies"))
	mustWrite(t, filepath.Join(src, "ShaderCache", "data"), []byte("shader"))
	mustWrite(t, filepath.Join(src, "Crashpad", "x"), []byte("crash"))
	mustWrite(t, filepath.Join(src, "SingletonLock"), []byte("lock"))
	mustWrite(t, filepath.Join(src, "Default", "LOG"), []byte("log"))
	link := filepath.Join(src, "Default", "SingletonCookie")
	if err := os.Symlink("ignored", link); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "user-data")
	stats, err := CopyUserData(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files < 5 {
		t.Fatalf("files copied = %d", stats.Files)
	}
	mustExist(t, filepath.Join(dst, "Default", "Preferences"))
	mustExist(t, filepath.Join(dst, "Default", "Local Storage", "leveldb", "CURRENT"))
	mustExist(t, filepath.Join(dst, "Default", "Service Worker", "Database", "CURRENT"))
	mustExist(t, filepath.Join(dst, "Default", "Local Extension Settings", "abc", "Cache", "data"))
	mustExist(t, filepath.Join(dst, "Default", "Network", "Cookies"))
	mustAbsent(t, filepath.Join(dst, "Default", "Cache"))
	mustAbsent(t, filepath.Join(dst, "Default", "Code Cache"))
	mustAbsent(t, filepath.Join(dst, "Default", "Service Worker", "CacheStorage"))
	mustAbsent(t, filepath.Join(dst, "Default", "Network", "Cache"))
	mustAbsent(t, filepath.Join(dst, "ShaderCache"))
	mustAbsent(t, filepath.Join(dst, "Crashpad"))
	mustAbsent(t, filepath.Join(dst, "SingletonLock"))
	mustAbsent(t, filepath.Join(dst, "Default", "LOG"))
	mustAbsent(t, filepath.Join(dst, "Default", "SingletonCookie"))

	measured, err := MeasureUserData(src)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Files != stats.Files || measured.Bytes != stats.Bytes {
		t.Fatalf("measure %+v copy %+v", measured, stats)
	}
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
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
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}
