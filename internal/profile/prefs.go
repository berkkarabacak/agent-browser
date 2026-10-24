package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"

	"abrowser/internal/jsonfile"
)

// PrepareUserData sets the Chromium profile display name and marks the
// profile as cleanly exited. Secure Preferences is removed only when a field
// changes, so Chromium regenerates the HMAC instead of restoring the old name.
func PrepareUserData(userDataDir, profileName string) error {
	if profileName == "" {
		return fmt.Errorf("profile name is empty")
	}
	defaultDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return err
	}
	prefsPath := filepath.Join(defaultDir, "Preferences")
	prefs, err := loadMap(prefsPath)
	if err != nil {
		return err
	}
	profile, err := objectField(prefs, "profile")
	if err != nil {
		return fmt.Errorf("preferences: %w", err)
	}
	prefsChanged := false
	if profile["name"] != profileName {
		profile["name"] = profileName
		profile["avatar_index"] = avatarIndex(profileName)
		prefsChanged = true
	} else if _, ok := profile["avatar_index"]; !ok {
		profile["avatar_index"] = avatarIndex(profileName)
		prefsChanged = true
	}
	if profile["exit_type"] != "Normal" {
		profile["exit_type"] = "Normal"
		prefsChanged = true
	}
	if profile["exited_cleanly"] != true {
		profile["exited_cleanly"] = true
		prefsChanged = true
	}

	localPath := filepath.Join(userDataDir, "Local State")
	local, err := loadMap(localPath)
	if err != nil {
		return err
	}
	localProfile, err := objectField(local, "profile")
	if err != nil {
		return fmt.Errorf("local state: %w", err)
	}
	infoCache, err := objectField(localProfile, "info_cache")
	if err != nil {
		return fmt.Errorf("local state: %w", err)
	}
	def, err := objectField(infoCache, "Default")
	if err != nil {
		return fmt.Errorf("local state: %w", err)
	}
	localChanged := false
	if def["name"] != profileName {
		def["name"] = profileName
		def["is_using_default_name"] = false
		localChanged = true
	}

	if prefsChanged {
		if err := jsonfile.Write(prefsPath, prefs); err != nil {
			return err
		}
	}
	if localChanged {
		if err := jsonfile.Write(localPath, local); err != nil {
			return err
		}
	}
	if prefsChanged || localChanged {
		secure := filepath.Join(defaultDir, "Secure Preferences")
		if err := os.Remove(secure); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func avatarIndex(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32() % 26)
}

func loadMap(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func objectField(m map[string]any, key string) (map[string]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		child := map[string]any{}
		m[key] = child
		return child, nil
	}
	child, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %q is not an object", key)
	}
	return child, nil
}
