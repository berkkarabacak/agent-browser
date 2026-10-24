package desktop

import (
	"sort"
	"sync"

	"abrowser/internal/app"
	"abrowser/internal/extensions"
)

// Snapshot is a consistent read of the data root for the window.
type Snapshot struct {
	Lines      []ProfileLine
	Templates  []TemplateLine
	Extensions []extensions.Extension
	Groups     map[string][]string
	GroupNames []string
	Config     app.ConfigView
	Status     app.Status
}

// TemplateLine is a golden template offered by the clone form.
type TemplateLine struct {
	Name   string
	Frozen bool
	Groups []string
}

// Service serializes calls into the core. The core lock file records this
// process id, so two overlapping App methods in one process deadlock.
type Service struct {
	mu   sync.Mutex
	core *app.App
}

// Open prepares a service for root. An empty root uses the platform default.
func Open(root string) (*Service, error) {
	core, err := app.Open(root)
	if err != nil {
		return nil, err
	}
	if err := core.Init(); err != nil {
		return nil, err
	}
	return &Service{core: core}, nil
}

// Load reads profiles, templates, extensions, groups, settings, and RAM.
func (s *Service) Load() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.core.Init(); err != nil {
		return Snapshot{}, err
	}
	inv, err := s.core.List()
	if err != nil {
		return Snapshot{}, err
	}
	st, err := s.core.Status()
	if err != nil {
		return Snapshot{}, err
	}
	cfg, err := s.core.ShowConfig()
	if err != nil {
		return Snapshot{}, err
	}
	exts, err := s.core.Extensions()
	if err != nil {
		return Snapshot{}, err
	}
	groups, err := s.core.Groups()
	if err != nil {
		return Snapshot{}, err
	}
	return assemble(inv, st, cfg, exts, groups), nil
}

func assemble(inv app.Inventory, st app.Status, cfg app.ConfigView, exts []extensions.Extension, groups extensions.GroupsFile) Snapshot {
	rss := map[string]app.StatusEntry{}
	for _, e := range st.Running {
		if e.Kind == "profile" {
			rss[e.ID] = e
		}
	}
	lines := make([]ProfileLine, 0, len(inv.Profiles))
	for _, p := range inv.Profiles {
		line := ProfileLine{
			ID:       p.ID,
			Email:    p.Email,
			Template: p.Template,
			Groups:   append([]string{}, p.Groups...),
			Add:      append([]string{}, p.AddExtensions...),
			Remove:   append([]string{}, p.RemoveExtensions...),
			Running:  p.Running,
		}
		if e, ok := rss[p.ID]; ok {
			line.RSS = e.RSS
			line.KnownRAM = true
			line.Running = true
		}
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ID < lines[j].ID })

	templates := make([]TemplateLine, 0, len(inv.Templates))
	for _, t := range inv.Templates {
		templates = append(templates, TemplateLine{
			Name:   t.Name,
			Frozen: t.Frozen,
			Groups: append([]string{}, t.Groups...),
		})
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })

	copied := map[string][]string{}
	names := make([]string, 0, len(groups))
	for name, members := range groups {
		names = append(names, name)
		copied[name] = append([]string{}, members...)
		sort.Strings(copied[name])
	}
	sort.Strings(names)
	if exts == nil {
		exts = []extensions.Extension{}
	}
	return Snapshot{
		Lines:      lines,
		Templates:  templates,
		Extensions: exts,
		Groups:     copied,
		GroupNames: names,
		Config:     cfg,
		Status:     st,
	}
}

// Launch starts one profile.
func (s *Service) Launch(id string) (app.LaunchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.LaunchProfile(id, false)
}

// Close stops one profile.
func (s *Service) Close(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.CloseProfile(id)
}

// Clone copies a frozen template.
func (s *Service) Clone(req app.CloneRequest) (app.CloneResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.Clone(req)
}

// Show loads one profile and its resolved extensions.
func (s *Service) Show(id string) (app.ProfileView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.ShowProfile(id)
}

// SetProfileGroups applies a group checkbox edit.
func (s *Service) SetProfileGroups(id string, have, want []string) error {
	add, remove := changedStrings(have, want)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range remove {
		if _, err := s.core.ProfileGroup(id, "remove", g); err != nil {
			return err
		}
	}
	for _, g := range add {
		if _, err := s.core.ProfileGroup(id, "add", g); err != nil {
			return err
		}
	}
	return nil
}

// ProfileExt edits a +ext or -ext override. op is add, remove, or reset.
func (s *Service) ProfileExt(id, op, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.core.ProfileExt(id, op, ref)
	return err
}

// AddExtension copies an unpacked directory or a CRX into the store.
func (s *Service) AddExtension(name, version, src string, crx bool) (extensions.Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if crx {
		return s.core.AddExtensionFromCRX(name, version, src)
	}
	return s.core.AddExtensionFromDir(name, version, src)
}

// RemoveExtension deletes one stored version. yes must be true.
func (s *Service) RemoveExtension(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.core.RemoveExtension(ref, true)
	return err
}

// CreateGroup adds an empty group.
func (s *Service) CreateGroup(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.CreateGroup(name)
}

// GroupAdd pins an extension into a group.
func (s *Service) GroupAdd(group, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.core.GroupAdd(group, ref)
	return err
}

// GroupRemove drops an extension from a group.
func (s *Service) GroupRemove(group, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.core.GroupRemove(group, ref)
}

// SaveSettings writes the browser path, the concurrent cap, and auto-close.
func (s *Service) SaveSettings(browser string, max int, autoClose bool) (app.ConfigView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.core.SetBrowser(browser); err != nil {
		return app.ConfigView{}, err
	}
	if _, err := s.core.SetMaxConcurrent(max); err != nil {
		return app.ConfigView{}, err
	}
	return s.core.SetAutoCloseLRU(autoClose)
}
