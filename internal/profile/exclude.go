package profile

import (
	"io/fs"
	"path"
	"strings"
)

// Cache and runtime files that should not be copied from a golden template.
// Directory names are matched anywhere in the tree except under extension
// settings, IndexedDB, and local/session storage, where a folder named
// "Cache" is profile data rather than Chromium's own disk cache.
var skipDirs = map[string]struct{}{
	"Cache":                               {},
	"Code Cache":                          {},
	"GPUCache":                            {},
	"ShaderCache":                         {},
	"GrShaderCache":                       {},
	"GraphiteDawnCache":                   {},
	"DawnGraphiteCache":                   {},
	"DawnWebGPUCache":                     {},
	"Crashpad":                            {},
	"Crash Reports":                       {},
	"BrowserMetrics":                      {},
	"component_crx_cache":                 {},
	"extensions_crx_cache":                {},
	"optimization_guide_hint_cache_store": {},
	"OnDeviceHeadSuggestModel":            {},
	"Safe Browsing":                       {},
	"Crowd Deny":                          {},
	"blob_storage":                        {},
	"File System":                         {},
	"Shared Dictionary":                   {},
	"CacheStorage":                        {},
	"ScriptCache":                         {},
	"OptimizationHints":                   {},
	"Module Info Cache":                   {},
}

// Singleton and crash-pad files must not be cloned. They make the copy look
// like a browser is already running, or they are metrics scratch files.
var skipFiles = map[string]struct{}{
	"SingletonLock":              {},
	"SingletonCookie":            {},
	"SingletonSocket":            {},
	"lockfile":                   {},
	"BrowserMetrics-spare.pma":   {},
	"CrashpadMetrics-active.pma": {},
}

var protectedPrefixes = []string{
	"Default/Local Extension Settings/",
	"Default/Sync Extension Settings/",
	"Default/Managed Extension Settings/",
	"Default/Extension State/",
	"Default/Extension Rules/",
	"Default/Extension Scripts/",
	"Default/Extensions/",
	"Default/IndexedDB/",
	"Default/Local Storage/",
	"Default/Session Storage/",
	"Default/databases/",
	"Default/Database/",
	"Default/Platform Notifications/",
}

// ShouldSkip reports whether a path relative to the user-data directory is
// left behind when a template is cloned. rel uses forward slashes.
func ShouldSkip(rel string, mode fs.FileMode) bool {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "." || rel == "" {
		return false
	}
	if mode&fs.ModeSymlink != 0 {
		return true
	}
	base := path.Base(rel)
	if _, ok := skipFiles[base]; ok {
		return true
	}
	if isProtected(rel) {
		return false
	}
	if base == "LOG" || base == "LOG.old" {
		return true
	}
	if mode&fs.ModeDir != 0 {
		if _, ok := skipDirs[base]; ok {
			return true
		}
	}
	return false
}

func isProtected(rel string) bool {
	for _, prefix := range protectedPrefixes {
		dir := strings.TrimSuffix(prefix, "/")
		if rel == dir || strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
