package app

import (
	"fmt"
	"os"
	"strings"

	"abrowser/internal/extensions"
	"abrowser/internal/layout"
	"abrowser/internal/manifest"
	"abrowser/internal/names"
	"abrowser/internal/profile"
)

// AddExtensionFromDir copies an unpacked extension into the shared store.
func (a *App) AddExtensionFromDir(name, version, src string) (extensions.Extension, error) {
	var ext extensions.Extension
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		ext, err = extensions.AddDir(a.Root, name, version, src)
		return err
	})
	return ext, err
}

// AddExtensionFromCRX unpacks a CRX2 or CRX3 file into the shared store.
func (a *App) AddExtensionFromCRX(name, version, src string) (extensions.Extension, error) {
	var ext extensions.Extension
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		ext, err = extensions.AddCRX(a.Root, name, version, src)
		return err
	})
	return ext, err
}

// Extensions returns the shared store, sorted by name and version.
func (a *App) Extensions() ([]extensions.Extension, error) {
	var list []extensions.Extension
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		list, err = a.extensionList()
		return err
	})
	return list, err
}

func (a *App) extensionList() ([]extensions.Extension, error) {
	list, err := extensions.Load(a.Root)
	if err != nil {
		return nil, err
	}
	sortExtensions(list)
	if list == nil {
		list = []extensions.Extension{}
	}
	return list, nil
}

func sortExtensions(list []extensions.Extension) {
	// Local sort keeps Load's order stable for equal versions.
	for i := 1; i < len(list); i++ {
		j := i
		for j > 0 && extLess(list[j], list[j-1]) {
			list[j], list[j-1] = list[j-1], list[j]
			j--
		}
	}
}

func extLess(a, b extensions.Extension) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return extensions.CompareVersion(a.Version, b.Version) < 0
}

// RemoveExtension deletes one stored version when nothing references it.
func (a *App) RemoveExtension(token string, yes bool) (extensions.Extension, error) {
	if !yes {
		return extensions.Extension{}, fmt.Errorf("refusing to remove %s without confirmation; re-run with --yes", token)
	}
	var removed extensions.Extension
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		catalog, err := extensions.Load(a.Root)
		if err != nil {
			return err
		}
		ext, err := pickExtension(catalog, token)
		if err != nil {
			return err
		}
		uses, err := a.extensionUses(ext.Name, ext.Version)
		if err != nil {
			return err
		}
		if len(uses) > 0 {
			return fmt.Errorf("extension %s is still used by %s", ext.Ref(), strings.Join(uses, ", "))
		}
		if err := extensions.Remove(a.Root, ext.Name, ext.Version); err != nil {
			return err
		}
		removed = ext
		return nil
	})
	return removed, err
}

func pickExtension(catalog []extensions.Extension, token string) (extensions.Extension, error) {
	name, version, pinned, err := names.ParseExtRef(token)
	if err != nil {
		// Allow a looser token through Resolve's lookup by trying Parse after a direct scan.
		resolved, rerr := extensions.Resolve(catalog, nil, nil, []string{token}, nil)
		if rerr != nil {
			return extensions.Extension{}, rerr
		}
		if len(resolved) != 1 {
			return extensions.Extension{}, fmt.Errorf("extension %q is ambiguous; pass name@version", token)
		}
		return resolved[0], nil
	}
	if !pinned {
		var matches []extensions.Extension
		for _, ext := range catalog {
			if ext.Name == name {
				matches = append(matches, ext)
			}
		}
		if len(matches) == 0 {
			return extensions.Extension{}, fmt.Errorf("unknown extension %q", token)
		}
		if len(matches) > 1 {
			return extensions.Extension{}, fmt.Errorf("extension %q has more than one version; pass name@version", name)
		}
		return matches[0], nil
	}
	for _, ext := range catalog {
		if ext.Name == name && ext.Version == version {
			return ext, nil
		}
	}
	return extensions.Extension{}, fmt.Errorf("unknown extension %q", token)
}

func (a *App) extensionUses(name, version string) ([]string, error) {
	var uses []string
	groups, err := extensions.LoadGroups(a.Root)
	if err != nil {
		return nil, err
	}
	groupNames := make([]string, 0, len(groups))
	for g := range groups {
		groupNames = append(groupNames, g)
	}
	sortStrings(groupNames)
	for _, g := range groupNames {
		if refsMatch(groups[g], name, version) {
			uses = append(uses, "group "+g)
		}
	}
	templates, err := listManifestDirs(layout.Templates(a.Root))
	if err != nil {
		return nil, err
	}
	for _, n := range templates {
		t, err := a.loadTemplate(n)
		if err != nil {
			return nil, err
		}
		if refsMatch(t.AddExtensions, name, version) {
			uses = append(uses, "template "+n+" +ext")
		}
		if refsMatch(t.RemoveExtensions, name, version) {
			uses = append(uses, "template "+n+" -ext")
		}
	}
	profiles, err := listManifestDirs(layout.Profiles(a.Root))
	if err != nil {
		return nil, err
	}
	for _, id := range profiles {
		p, err := a.loadProfile(id)
		if err != nil {
			return nil, err
		}
		if refsMatch(p.AddExtensions, name, version) {
			uses = append(uses, "profile "+id+" +ext")
		}
		if refsMatch(p.RemoveExtensions, name, version) {
			uses = append(uses, "profile "+id+" -ext")
		}
	}
	return uses, nil
}

func refsMatch(list []string, name, version string) bool {
	for _, item := range list {
		n, v, pinned, err := names.ParseExtRef(item)
		if err != nil || n != name {
			continue
		}
		if !pinned || v == version {
			return true
		}
	}
	return false
}

// Groups returns the group map.
func (a *App) Groups() (extensions.GroupsFile, error) {
	var g extensions.GroupsFile
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		g, err = extensions.LoadGroups(a.Root)
		return err
	})
	return g, err
}

// CreateGroup adds an empty group.
func (a *App) CreateGroup(name string) error {
	if err := names.ValidGroup(name); err != nil {
		return err
	}
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		g, err := extensions.LoadGroups(a.Root)
		if err != nil {
			return err
		}
		if _, ok := g[name]; ok {
			return fmt.Errorf("group %q already exists", name)
		}
		g[name] = []string{}
		return extensions.SaveGroups(a.Root, g)
	})
}

// GroupChange is the result of adding an extension to a group.
type GroupChange struct {
	Group     string `json:"group"`
	Extension string `json:"extension"`
	Already   bool   `json:"already"`
}

// GroupAdd pins an extension into a group, replacing an older version of the same name.
func (a *App) GroupAdd(group, ref string) (GroupChange, error) {
	var change GroupChange
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if err := names.ValidGroup(group); err != nil {
			return err
		}
		g, err := extensions.LoadGroups(a.Root)
		if err != nil {
			return err
		}
		if _, ok := g[group]; !ok {
			return fmt.Errorf("unknown extension group %q", group)
		}
		catalog, err := extensions.Load(a.Root)
		if err != nil {
			return err
		}
		pinned, err := pinRef(catalog, ref)
		if err != nil {
			return err
		}
		next, already := upsertRef(g[group], pinned)
		g[group] = next
		if err := extensions.SaveGroups(a.Root, g); err != nil {
			return err
		}
		change = GroupChange{Group: group, Extension: pinned, Already: already}
		return nil
	})
	return change, err
}

// GroupRemove drops an extension from a group. A bare name removes every version of that name.
func (a *App) GroupRemove(group, ref string) error {
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		g, err := extensions.LoadGroups(a.Root)
		if err != nil {
			return err
		}
		list, ok := g[group]
		if !ok {
			return fmt.Errorf("unknown extension group %q", group)
		}
		next, err := removeRef(list, ref)
		if err != nil {
			return fmt.Errorf("group %s: %w", group, err)
		}
		g[group] = next
		return extensions.SaveGroups(a.Root, g)
	})
}

func pinRef(catalog []extensions.Extension, ref string) (string, error) {
	resolved, err := extensions.Resolve(catalog, nil, nil, []string{ref}, nil)
	if err != nil {
		return "", err
	}
	if len(resolved) != 1 {
		return "", fmt.Errorf("could not resolve %q", ref)
	}
	return resolved[0].Ref(), nil
}

func upsertRef(list []string, pinned string) ([]string, bool) {
	name, _, _ := strings.Cut(pinned, "@")
	out := make([]string, 0, len(list)+1)
	found := false
	already := false
	for _, item := range list {
		n, _, ok := strings.Cut(item, "@")
		if !ok {
			n = item
		}
		if n == name {
			if item == pinned && !found {
				already = true
			}
			if !found {
				out = append(out, pinned)
				found = true
			}
			continue
		}
		out = append(out, item)
	}
	if !found {
		out = append(out, pinned)
	}
	return out, already
}

func removeRef(list []string, token string) ([]string, error) {
	name, version, pinned, err := names.ParseExtRef(token)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	removed := false
	for _, item := range list {
		n, v, _, err := names.ParseExtRef(item)
		if err != nil {
			out = append(out, item)
			continue
		}
		if n == name && (!pinned || v == version) {
			removed = true
			continue
		}
		out = append(out, item)
	}
	if !removed {
		return nil, fmt.Errorf("does not contain %s", token)
	}
	return out, nil
}

// TemplateCreate is the input for a new golden template.
type TemplateCreate struct {
	Name     string
	Timezone string
	Locale   string
	Notes    string
	Groups   []string
}

// CreateTemplate creates an empty user-data directory and a manifest.
func (a *App) CreateTemplate(in TemplateCreate) (manifest.Template, error) {
	var created manifest.Template
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if err := names.ValidID(in.Name); err != nil {
			return err
		}
		if err := names.ValidTimezone(in.Timezone); err != nil {
			return err
		}
		locale := in.Locale
		if locale == "" {
			locale = "en-US"
		}
		if err := names.ValidLocale(locale); err != nil {
			return err
		}
		if _, err := a.loadTemplate(in.Name); err == nil {
			return fmt.Errorf("template %q already exists", in.Name)
		}
		if err := a.requireGroups(in.Groups); err != nil {
			return err
		}
		if _, err := a.resolve(in.Groups, nil, nil); err != nil {
			return err
		}
		now := a.now()
		t := manifest.Template{
			Name:             in.Name,
			Groups:           copyStrings(in.Groups),
			AddExtensions:    []string{},
			RemoveExtensions: []string{},
			Timezone:         in.Timezone,
			Locale:           locale,
			Notes:            in.Notes,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := os.MkdirAll(layout.TemplateUserData(a.Root, in.Name), 0o755); err != nil {
			return err
		}
		if err := profile.PrepareUserData(layout.TemplateUserData(a.Root, in.Name), in.Name); err != nil {
			_ = os.RemoveAll(layout.Template(a.Root, in.Name))
			return err
		}
		if err := a.saveTemplate(t); err != nil {
			_ = os.RemoveAll(layout.Template(a.Root, in.Name))
			return err
		}
		created = t
		return nil
	})
	return created, err
}

// FreezeResult describes a template after it was frozen.
type FreezeResult struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// FreezeTemplate marks a template immutable and reports the slim user-data size.
func (a *App) FreezeTemplate(name string) (FreezeResult, error) {
	var res FreezeResult
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		if t.Frozen {
			return fmt.Errorf("template %q is already frozen", name)
		}
		if st, ok, err := a.readRunning(layout.TemplateRunning(a.Root, name)); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("template %s is running (pid %d); close the window or run \"abrowser template close %s\" before freezing", name, st.PID, name)
		}
		stats, err := profile.MeasureUserData(layout.TemplateUserData(a.Root, name))
		if err != nil {
			return err
		}
		now := a.now()
		t.Frozen = true
		t.FrozenAt = &now
		t.UpdatedAt = now
		if err := a.saveTemplate(t); err != nil {
			return err
		}
		res = FreezeResult{Name: name, Files: stats.Files, Bytes: stats.Bytes}
		return nil
	})
	return res, err
}

// UnfreezeTemplate allows the golden profile to be launched and edited again.
func (a *App) UnfreezeTemplate(name string) error {
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		if !t.Frozen {
			return fmt.Errorf("template %q is not frozen", name)
		}
		if st, ok, err := a.readRunning(layout.TemplateRunning(a.Root, name)); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("template %s is running (pid %d)", name, st.PID)
		}
		t.Frozen = false
		t.FrozenAt = nil
		t.UpdatedAt = a.now()
		return a.saveTemplate(t)
	})
}

// FieldUpdates patches manifest fields. Nil pointers are left unchanged.
type FieldUpdates struct {
	Email    *string
	Timezone *string
	Locale   *string
	Notes    *string
}

func (u FieldUpdates) empty() bool {
	return u.Email == nil && u.Timezone == nil && u.Locale == nil && u.Notes == nil
}

// UpdateTemplate edits timezone, locale, or notes on a template.
func (a *App) UpdateTemplate(name string, u FieldUpdates) (manifest.Template, error) {
	var out manifest.Template
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if u.empty() {
			return fmt.Errorf("nothing to update")
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		if u.Timezone != nil {
			if err := names.ValidTimezone(*u.Timezone); err != nil {
				return err
			}
			t.Timezone = *u.Timezone
		}
		if u.Locale != nil {
			if *u.Locale != "" {
				if err := names.ValidLocale(*u.Locale); err != nil {
					return err
				}
			}
			t.Locale = *u.Locale
		}
		if u.Notes != nil {
			t.Notes = *u.Notes
		}
		t.UpdatedAt = a.now()
		if err := a.saveTemplate(t); err != nil {
			return err
		}
		out = t
		return nil
	})
	return out, err
}

// UpdateProfile edits email, timezone, locale, or notes.
func (a *App) UpdateProfile(id string, u FieldUpdates) (manifest.Profile, error) {
	var out manifest.Profile
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if u.empty() {
			return fmt.Errorf("nothing to update")
		}
		p, err := a.loadProfile(id)
		if err != nil {
			return err
		}
		if u.Email != nil {
			if err := names.ValidEmail(*u.Email); err != nil {
				return err
			}
			p.Email = *u.Email
		}
		if u.Timezone != nil {
			if err := names.ValidTimezone(*u.Timezone); err != nil {
				return err
			}
			p.Timezone = *u.Timezone
		}
		if u.Locale != nil {
			if *u.Locale != "" {
				if err := names.ValidLocale(*u.Locale); err != nil {
					return err
				}
			}
			p.Locale = *u.Locale
		}
		if u.Notes != nil {
			p.Notes = *u.Notes
		}
		p.UpdatedAt = a.now()
		if err := a.saveProfile(p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}

// TemplateGroup adds or removes a group on a template.
func (a *App) TemplateGroup(name, op, group string) (manifest.Template, error) {
	var out manifest.Template
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		next, err := a.applyGroupOp(t.Groups, op, group)
		if err != nil {
			return err
		}
		t.Groups = next
		if _, err := a.resolve(t.Groups, t.AddExtensions, t.RemoveExtensions); err != nil {
			return err
		}
		t.UpdatedAt = a.now()
		if err := a.saveTemplate(t); err != nil {
			return err
		}
		out = t
		return nil
	})
	return out, err
}

// ProfileGroup adds or removes a group on a profile.
func (a *App) ProfileGroup(id, op, group string) (manifest.Profile, error) {
	var out manifest.Profile
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		p, err := a.loadProfile(id)
		if err != nil {
			return err
		}
		next, err := a.applyGroupOp(p.Groups, op, group)
		if err != nil {
			return err
		}
		p.Groups = next
		if _, err := a.resolve(p.Groups, p.AddExtensions, p.RemoveExtensions); err != nil {
			return err
		}
		p.UpdatedAt = a.now()
		if err := a.saveProfile(p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}

func (a *App) applyGroupOp(groups []string, op, group string) ([]string, error) {
	if err := names.ValidGroup(group); err != nil {
		return nil, err
	}
	known, err := extensions.LoadGroups(a.Root)
	if err != nil {
		return nil, err
	}
	switch op {
	case "add":
		if _, ok := known[group]; !ok {
			return nil, fmt.Errorf("unknown extension group %q", group)
		}
		for _, existing := range groups {
			if existing == group {
				return copyStrings(groups), nil
			}
		}
		return append(copyStrings(groups), group), nil
	case "remove":
		out := make([]string, 0, len(groups))
		found := false
		for _, existing := range groups {
			if existing == group {
				found = true
				continue
			}
			out = append(out, existing)
		}
		if !found {
			return nil, fmt.Errorf("group %q is not assigned", group)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown group operation %q", op)
	}
}

// TemplateExt edits +ext / -ext overrides. op is add, remove, or reset.
func (a *App) TemplateExt(name, op, ref string) (manifest.Template, error) {
	var out manifest.Template
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		catalog, err := extensions.Load(a.Root)
		if err != nil {
			return err
		}
		add, remove, err := applyExtOp(t.AddExtensions, t.RemoveExtensions, op, ref, catalog)
		if err != nil {
			return err
		}
		if _, err := a.resolve(t.Groups, add, remove); err != nil {
			return err
		}
		t.AddExtensions = add
		t.RemoveExtensions = remove
		t.UpdatedAt = a.now()
		if err := a.saveTemplate(t); err != nil {
			return err
		}
		out = t
		return nil
	})
	return out, err
}

// ProfileExt edits +ext / -ext overrides. op is add, remove, or reset.
func (a *App) ProfileExt(id, op, ref string) (manifest.Profile, error) {
	var out manifest.Profile
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		p, err := a.loadProfile(id)
		if err != nil {
			return err
		}
		catalog, err := extensions.Load(a.Root)
		if err != nil {
			return err
		}
		add, remove, err := applyExtOp(p.AddExtensions, p.RemoveExtensions, op, ref, catalog)
		if err != nil {
			return err
		}
		if _, err := a.resolve(p.Groups, add, remove); err != nil {
			return err
		}
		p.AddExtensions = add
		p.RemoveExtensions = remove
		p.UpdatedAt = a.now()
		if err := a.saveProfile(p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}

func applyExtOp(add, remove []string, op, ref string, catalog []extensions.Extension) ([]string, []string, error) {
	switch op {
	case "add":
		pinned, err := pinRef(catalog, ref)
		if err != nil {
			return nil, nil, err
		}
		name, _, _ := strings.Cut(pinned, "@")
		remove, _ = dropName(remove, name, "")
		add, _ = upsertRef(add, pinned)
		return add, remove, nil
	case "remove":
		pinned, err := pinRef(catalog, ref)
		if err != nil {
			return nil, nil, err
		}
		name, _, _ := strings.Cut(pinned, "@")
		add, _ = dropName(add, name, "")
		remove, _ = upsertRef(remove, pinned)
		return add, remove, nil
	case "reset":
		name, version, pinned, err := names.ParseExtRef(ref)
		if err != nil {
			return nil, nil, err
		}
		var addChanged, removeChanged bool
		if pinned {
			add, addChanged = dropName(add, name, version)
			remove, removeChanged = dropName(remove, name, version)
		} else {
			add, addChanged = dropName(add, name, "")
			remove, removeChanged = dropName(remove, name, "")
		}
		if !addChanged && !removeChanged {
			return nil, nil, fmt.Errorf("no override for %s", ref)
		}
		return add, remove, nil
	default:
		return nil, nil, fmt.Errorf("unknown extension operation %q", op)
	}
}

func dropName(list []string, name, version string) ([]string, bool) {
	out := make([]string, 0, len(list))
	changed := false
	for _, item := range list {
		n, v, _, err := names.ParseExtRef(item)
		if err == nil && n == name && (version == "" || v == version) {
			changed = true
			continue
		}
		out = append(out, item)
	}
	return out, changed
}

// CloneRequest copies a frozen template into a new profile.
type CloneRequest struct {
	Template    string
	ID          string
	Email       string
	Timezone    *string
	Locale      *string
	Notes       *string
	Groups      []string
	GroupsSet   bool
	Add         []string
	AddSet      bool
	Remove      []string
	RemoveSet   bool
	DisplayName string
}

// CloneResult is the new profile and how much user data was copied.
type CloneResult struct {
	Profile manifest.Profile `json:"profile"`
	Files   int              `json:"files"`
	Bytes   int64            `json:"bytes"`
}

// Clone copies a frozen template's user-data directory, skipping caches.
func (a *App) Clone(req CloneRequest) (CloneResult, error) {
	var res CloneResult
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if err := names.ValidID(req.ID); err != nil {
			return err
		}
		if err := names.ValidEmail(req.Email); err != nil {
			return err
		}
		t, err := a.loadTemplate(req.Template)
		if err != nil {
			return err
		}
		if !t.Frozen {
			return fmt.Errorf("template %q is not frozen; configure it, close it, then run: abrowser template freeze %s", req.Template, req.Template)
		}
		if st, ok, err := a.readRunning(layout.TemplateRunning(a.Root, req.Template)); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("template %s is running (pid %d); close it before cloning", req.Template, st.PID)
		}
		if _, err := a.loadProfile(req.ID); err == nil {
			return fmt.Errorf("profile %q already exists", req.ID)
		}
		groups := copyStrings(t.Groups)
		if req.GroupsSet {
			groups = copyStrings(req.Groups)
		}
		add := copyStrings(t.AddExtensions)
		if req.AddSet {
			add = copyStrings(req.Add)
		}
		remove := copyStrings(t.RemoveExtensions)
		if req.RemoveSet {
			remove = copyStrings(req.Remove)
		}
		tz := t.Timezone
		if req.Timezone != nil {
			tz = *req.Timezone
		}
		locale := t.Locale
		if req.Locale != nil {
			locale = *req.Locale
		}
		notes := t.Notes
		if req.Notes != nil {
			notes = *req.Notes
		}
		if err := names.ValidTimezone(tz); err != nil {
			return err
		}
		if locale != "" {
			if err := names.ValidLocale(locale); err != nil {
				return err
			}
		}
		if err := a.requireGroups(groups); err != nil {
			return err
		}
		if _, err := a.resolve(groups, add, remove); err != nil {
			return err
		}
		destRoot := layout.Profile(a.Root, req.ID)
		if _, err := os.Stat(destRoot); err == nil {
			return fmt.Errorf("profile %q already exists", req.ID)
		}
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(destRoot)
			}
		}()
		stats, err := profile.CopyUserData(layout.TemplateUserData(a.Root, req.Template), layout.ProfileUserData(a.Root, req.ID))
		if err != nil {
			return err
		}
		if err := profile.PrepareUserData(layout.ProfileUserData(a.Root, req.ID), req.ID); err != nil {
			return err
		}
		now := a.now()
		p := manifest.Profile{
			ID:               req.ID,
			DisplayName:      trimDisplayName(req.DisplayName),
			Email:            req.Email,
			Template:         req.Template,
			Groups:           groups,
			AddExtensions:    add,
			RemoveExtensions: remove,
			Timezone:         tz,
			Locale:           locale,
			Notes:            notes,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := a.saveProfile(p); err != nil {
			return err
		}
		committed = true
		res = CloneResult{Profile: p, Files: stats.Files, Bytes: stats.Bytes}
		return nil
	})
	return res, err
}

func trimDisplayName(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 80 {
		r = r[:80]
	}
	return string(r)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j] < s[j-1] {
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}
