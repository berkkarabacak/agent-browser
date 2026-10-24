// Package extensions stores unpacked Chromium extensions and resolves
// the set a profile should load.
package extensions

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"abrowser/internal/jsonfile"
	"abrowser/internal/layout"
	"abrowser/internal/names"
)

// Extension is one unpacked version in the shared store.
type Extension struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Path            string `json:"path"`
	ManifestName    string `json:"manifestName"`
	ManifestVersion int    `json:"manifestVersion"`
	Description     string `json:"description,omitempty"`
}

// Ref returns the name@version token stored in groups and overrides.
func (e Extension) Ref() string { return e.Name + "@" + e.Version }

type extManifest struct {
	ManifestVersion int    `json:"manifest_version"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Description     string `json:"description"`
}

// CompareVersion returns -1 if a < b, 0 if equal, and 1 if a > b.
// Dot-separated components are compared as integers when both are integers.
// A missing component compares equal to 0, so 1.2 equals 1.2.0.
func CompareVersion(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ap, bp := "0", "0"
		if i < len(as) {
			ap = as[i]
		}
		if i < len(bs) {
			bp = bs[i]
		}
		ai, aErr := strconv.Atoi(ap)
		bi, bErr := strconv.Atoi(bp)
		if aErr == nil && bErr == nil {
			switch {
			case ai < bi:
				return -1
			case ai > bi:
				return 1
			}
			continue
		}
		if ap < bp {
			return -1
		}
		if ap > bp {
			return 1
		}
	}
	return 0
}

// Load reads every name@version directory under the root.
func Load(root string) ([]Extension, error) {
	dir := layout.Extensions(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Extension
	seen := map[string]struct{}{}
	for _, ent := range entries {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		name, version, ok := strings.Cut(ent.Name(), "@")
		if !ok || name == "" || version == "" {
			continue
		}
		ext, err := readInstalled(root, name, version)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[ext.Ref()]; dup {
			return nil, fmt.Errorf("duplicate extension %s", ext.Ref())
		}
		seen[ext.Ref()] = struct{}{}
		out = append(out, ext)
	}
	return out, nil
}

func readInstalled(root, name, version string) (Extension, error) {
	dir := layout.Extension(root, name, version)
	mf, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Extension{}, fmt.Errorf("extension %s@%s: %w", name, version, err)
	}
	if mf.Version != version {
		return Extension{}, fmt.Errorf("extension %s@%s: manifest version is %s", name, version, mf.Version)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Extension{}, err
	}
	return Extension{
		Name:            name,
		Version:         version,
		Path:            abs,
		ManifestName:    mf.Name,
		ManifestVersion: mf.ManifestVersion,
		Description:     mf.Description,
	}, nil
}

func readManifest(path string) (extManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return extManifest{}, err
	}
	var mf extManifest
	if err := json.Unmarshal(b, &mf); err != nil {
		return extManifest{}, fmt.Errorf("manifest.json: %w", err)
	}
	if mf.ManifestVersion != 2 && mf.ManifestVersion != 3 {
		return extManifest{}, fmt.Errorf("manifest_version must be 2 or 3")
	}
	if strings.TrimSpace(mf.Name) == "" {
		return extManifest{}, fmt.Errorf("manifest name is empty")
	}
	if err := names.ValidVersion(mf.Version); err != nil {
		return extManifest{}, err
	}
	return mf, nil
}

// AddDir copies an unpacked extension into the store.
// version is optional; the manifest version is used when it is empty.
// When both are set they must match.
func AddDir(root, name, version, src string) (Extension, error) {
	if err := names.ValidExtName(name); err != nil {
		return Extension{}, err
	}
	mf, err := readManifest(filepath.Join(src, "manifest.json"))
	if err != nil {
		return Extension{}, fmt.Errorf("reading %s: %w", src, err)
	}
	version, err = chooseVersion(version, mf.Version)
	if err != nil {
		return Extension{}, err
	}
	dest := layout.Extension(root, name, version)
	if _, err := os.Stat(dest); err == nil {
		return Extension{}, fmt.Errorf("extension %s@%s already exists", name, version)
	}
	if err := os.MkdirAll(layout.Extensions(root), 0o755); err != nil {
		return Extension{}, err
	}
	if err := copyTree(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return Extension{}, err
	}
	return readInstalled(root, name, version)
}

// AddCRX unpacks a CRX2 or CRX3 file into the store.
func AddCRX(root, name, version, crxPath string) (Extension, error) {
	if err := names.ValidExtName(name); err != nil {
		return Extension{}, err
	}
	tmp, err := os.MkdirTemp(root, ".crx-*")
	if err != nil {
		return Extension{}, err
	}
	defer os.RemoveAll(tmp)
	unpacked := filepath.Join(tmp, "unpacked")
	if err := ExtractCRX(crxPath, unpacked); err != nil {
		return Extension{}, err
	}
	return AddDir(root, name, version, unpacked)
}

// Remove deletes one version directory.
func Remove(root, name, version string) error {
	dir := layout.Extension(root, name, version)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("extension %s@%s is not in the store", name, version)
		}
		return err
	}
	return os.RemoveAll(dir)
}

func chooseVersion(flag, manifest string) (string, error) {
	if flag == "" {
		return manifest, nil
	}
	if err := names.ValidVersion(flag); err != nil {
		return "", err
	}
	if flag != manifest {
		return "", fmt.Errorf("version %s does not match manifest version %s", flag, manifest)
	}
	return manifest, nil
}

func copyTree(src, dst string) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(srcAbs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.WalkDir(srcAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("symlink %s: %w", rel, err)
			}
			if !within(srcAbs, resolved) {
				return fmt.Errorf("symlink %s points outside the extension", rel)
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			return copyFile(path, target, info.Mode())
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func within(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm()&0o755|0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// GroupsFile is the on-disk group map. Keys are group names.
type GroupsFile map[string][]string

// LoadGroups reads groups.json. A missing file is an empty map.
func LoadGroups(root string) (GroupsFile, error) {
	path := layout.Groups(root)
	g := GroupsFile{}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return g, nil
		}
		return nil, err
	}
	if err := jsonfile.Read(path, &g); err != nil {
		return nil, err
	}
	if g == nil {
		g = GroupsFile{}
	}
	for name, refs := range g {
		if refs == nil {
			g[name] = []string{}
		}
	}
	return g, nil
}

// SaveGroups writes groups.json.
func SaveGroups(root string, g GroupsFile) error {
	if g == nil {
		g = GroupsFile{}
	}
	return jsonfile.Write(layout.Groups(root), g)
}
