package extensions

import (
	"fmt"
	"sort"
	"strings"
)

// Resolve builds the ordered extension list for a profile.
//
// Group members are added first, in group order, then addRefs.
// Duplicates keep the earliest position. removeRefs are applied last, so a
// removal wins over a group membership and over an add.
// A remove ref that is not in the catalog is an error. A remove ref that is
// in the catalog but not selected is allowed.
func Resolve(catalog []Extension, groups map[string][]string, groupNames, addRefs, removeRefs []string) ([]Extension, error) {
	byRef := map[string]Extension{}
	byName := map[string][]Extension{}
	for _, ext := range catalog {
		ref := ext.Ref()
		if _, ok := byRef[ref]; ok {
			return nil, fmt.Errorf("duplicate extension %s in catalog", ref)
		}
		byRef[ref] = ext
		byName[ext.Name] = append(byName[ext.Name], ext)
	}
	for name := range byName {
		vers := byName[name]
		sort.SliceStable(vers, func(i, j int) bool {
			return CompareVersion(vers[i].Version, vers[j].Version) < 0
		})
		byName[name] = vers
	}

	lookup := func(token string) (Extension, error) {
		name, version, pinned, err := splitRef(token)
		if err != nil {
			return Extension{}, err
		}
		if pinned {
			ext, ok := byRef[name+"@"+version]
			if !ok {
				return Extension{}, fmt.Errorf("unknown extension %q", token)
			}
			return ext, nil
		}
		vers := byName[name]
		if len(vers) == 0 {
			return Extension{}, fmt.Errorf("unknown extension %q", token)
		}
		return vers[len(vers)-1], nil
	}

	type removal struct {
		name    string
		version string
		exact   bool
	}
	var removals []removal
	for _, token := range removeRefs {
		name, version, pinned, err := splitRef(token)
		if err != nil {
			return nil, err
		}
		if _, err := lookup(token); err != nil {
			return nil, fmt.Errorf("remove: %w", err)
		}
		removals = append(removals, removal{name: name, version: version, exact: pinned})
	}

	var ordered []Extension
	seen := map[string]bool{}
	add := func(ext Extension) {
		if seen[ext.Ref()] {
			return
		}
		seen[ext.Ref()] = true
		ordered = append(ordered, ext)
	}

	for _, group := range groupNames {
		members, ok := groups[group]
		if !ok {
			return nil, fmt.Errorf("unknown extension group %q", group)
		}
		for _, token := range members {
			ext, err := lookup(token)
			if err != nil {
				return nil, fmt.Errorf("group %q: %w", group, err)
			}
			add(ext)
		}
	}
	for _, token := range addRefs {
		ext, err := lookup(token)
		if err != nil {
			return nil, fmt.Errorf("add: %w", err)
		}
		add(ext)
	}

	filtered := make([]Extension, 0, len(ordered))
	for _, ext := range ordered {
		drop := false
		for _, rem := range removals {
			if ext.Name != rem.name {
				continue
			}
			if !rem.exact || ext.Version == rem.version {
				drop = true
				break
			}
		}
		if !drop {
			filtered = append(filtered, ext)
		}
	}
	return filtered, nil
}

func splitRef(token string) (name, version string, pinned bool, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", "", false, fmt.Errorf("empty extension reference")
	}
	name, version, pinned = strings.Cut(token, "@")
	if name == "" || (pinned && version == "") {
		return "", "", false, fmt.Errorf("invalid extension reference %q", token)
	}
	if strings.Contains(version, "@") {
		return "", "", false, fmt.Errorf("invalid extension reference %q", token)
	}
	return name, version, pinned, nil
}
