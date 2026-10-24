// Package layout maps the on-disk Agent Browser root.
package layout

import (
	"os"
	"path/filepath"
	"runtime"
)

const dirName = "AgentBrowser"

// DefaultRoot is %LOCALAPPDATA%\AgentBrowser on Windows and
// $XDG_DATA_HOME/AgentBrowser or ~/.local/share/AgentBrowser elsewhere.
func DefaultRoot() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				base = filepath.Join(home, "AppData", "Local")
			}
		}
		if base == "" {
			base = `C:\Users\Default\AppData\Local`
		}
		return filepath.Join(base, dirName)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, dirName)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(string(filepath.Separator), "tmp", dirName)
	}
	return filepath.Join(home, ".local", "share", dirName)
}

func Config(root string) string   { return filepath.Join(root, "config.json") }
func Groups(root string) string   { return filepath.Join(root, "groups.json") }
func Registry(root string) string { return filepath.Join(root, "registry.json") }
func Lock(root string) string     { return filepath.Join(root, ".abrowser.lock") }

func Extensions(root string) string { return filepath.Join(root, "extensions") }
func Extension(root, name, version string) string {
	return filepath.Join(root, "extensions", name+"@"+version)
}

func Templates(root string) string { return filepath.Join(root, "templates") }
func Template(root, name string) string {
	return filepath.Join(root, "templates", name)
}
func TemplateManifest(root, name string) string {
	return filepath.Join(Template(root, name), "manifest.json")
}
func TemplateUserData(root, name string) string {
	return filepath.Join(Template(root, name), "user-data")
}
func TemplateRunning(root, name string) string {
	return filepath.Join(Template(root, name), "running.json")
}
func TemplateBadge(root, name string) string {
	return filepath.Join(Template(root, name), "badge")
}
func TemplateLog(root, name string) string {
	return filepath.Join(Template(root, name), "launch.log")
}

func Profiles(root string) string { return filepath.Join(root, "profiles") }
func Profile(root, id string) string {
	return filepath.Join(root, "profiles", id)
}
func ProfileManifest(root, id string) string {
	return filepath.Join(Profile(root, id), "manifest.json")
}
func ProfileUserData(root, id string) string {
	return filepath.Join(Profile(root, id), "user-data")
}
func ProfileRunning(root, id string) string {
	return filepath.Join(Profile(root, id), "running.json")
}
func ProfileBadge(root, id string) string {
	return filepath.Join(Profile(root, id), "badge")
}
func ProfileLog(root, id string) string {
	return filepath.Join(Profile(root, id), "launch.log")
}
