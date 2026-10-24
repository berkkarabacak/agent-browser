package profile

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ArgOptions is the Chromium command line for one profile.
type ArgOptions struct {
	UserDataDir    string
	ExtensionPaths []string
	Locale         string
	Extra          []string
	ClassName      string
	GOOS           string
}

// BuildArgs returns Chromium arguments. Extension paths are absolute.
// A comma inside a path is rejected because --load-extension splits on commas.
func BuildArgs(opts ArgOptions) ([]string, error) {
	if strings.TrimSpace(opts.UserDataDir) == "" {
		return nil, fmt.Errorf("user-data-dir is empty")
	}
	userData, err := filepath.Abs(opts.UserDataDir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range opts.ExtensionPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if strings.Contains(abs, ",") {
			return nil, fmt.Errorf("extension path %q contains a comma; Chromium splits --load-extension on commas", abs)
		}
		paths = append(paths, abs)
	}
	args := []string{
		"--user-data-dir=" + userData,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--disable-default-apps",
		"--new-window",
	}
	if len(paths) > 0 {
		joined := strings.Join(paths, ",")
		args = append(args,
			"--disable-extensions-except="+joined,
			"--load-extension="+joined,
		)
	}
	if opts.Locale != "" {
		args = append(args, "--lang="+opts.Locale, "--accept-lang="+opts.Locale)
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "linux" && opts.ClassName != "" {
		args = append(args, "--class="+opts.ClassName)
	}
	args = append(args, opts.Extra...)
	return args, nil
}

// BuildEnv returns the process environment for a launch.
// A non-empty timezone replaces TZ. locale is exported as LANGUAGE.
// Other variables are preserved.
func BuildEnv(base []string, timezone, locale string) []string {
	overrides := map[string]string{}
	if timezone != "" {
		overrides["TZ"] = timezone
	}
	if locale != "" {
		overrides["LANGUAGE"] = locale
	}
	return mergeEnv(base, overrides)
}

func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replace := overrides[key]; replace {
			continue
		}
		out = append(out, entry)
	}
	keys := make([]string, 0, len(overrides))
	for key, val := range overrides {
		if val == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+overrides[key])
	}
	return out
}

// ClassName is the X11/Wayland window class used on Linux.
func ClassName(id string) string { return "abrowser-" + id }
