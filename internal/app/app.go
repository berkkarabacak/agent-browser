// Package app implements Agent Browser operations shared by the CLI.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"abrowser/internal/extensions"
	"abrowser/internal/jsonfile"
	"abrowser/internal/layout"
	"abrowser/internal/manifest"
	"abrowser/internal/names"
	"abrowser/internal/proc"
)

const (
	defaultMaxConcurrent = 20
	maxConcurrentLimit   = 500
	registryVersion      = 1
)

// App is a data root plus the operations that mutate it.
type App struct {
	Root  string
	Now   func() time.Time
	Grace time.Duration
}

// NotRunningError means close was asked to stop a browser that is not running.
type NotRunningError struct {
	Kind string
	ID   string
}

func (e *NotRunningError) Error() string {
	kind := e.Kind
	if kind == "" {
		kind = "profile"
	}
	return kind + " " + e.ID + " is not running"
}

// Open returns an App for root. An empty root uses the platform default.
func Open(root string) (*App, error) {
	if strings.TrimSpace(root) == "" {
		root = layout.DefaultRoot()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &App{Root: abs, Grace: 350 * time.Millisecond}, nil
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC()
	}
	return time.Now().UTC()
}

func (a *App) grace() time.Duration {
	if a.Grace > 0 {
		return a.Grace
	}
	return 350 * time.Millisecond
}

// Init creates the data root if needed.
func (a *App) Init() error {
	return a.withLock(func() error { return a.ensure() })
}

func (a *App) ensure() error {
	for _, dir := range []string{
		a.Root,
		layout.Extensions(a.Root),
		layout.Templates(a.Root),
		layout.Profiles(a.Root),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(layout.Config(a.Root)); os.IsNotExist(err) {
		if err := a.saveConfig(Config{MaxConcurrent: defaultMaxConcurrent, ExtraArgs: []string{}}); err != nil {
			return err
		}
	}
	if _, err := os.Stat(layout.Groups(a.Root)); os.IsNotExist(err) {
		if err := extensions.SaveGroups(a.Root, extensions.GroupsFile{}); err != nil {
			return err
		}
	}
	if _, err := os.Stat(layout.Registry(a.Root)); os.IsNotExist(err) {
		if err := a.rebuildRegistry(); err != nil {
			return err
		}
	}
	return nil
}

// Config is the root config.json document.
type Config struct {
	BrowserPath   string   `json:"browserPath"`
	MaxConcurrent int      `json:"maxConcurrent"`
	AutoCloseLRU  bool     `json:"autoCloseLRU"`
	ExtraArgs     []string `json:"extraArgs"`
	SetupComplete bool     `json:"setupComplete"`
}

// ConfigView is config plus derived path checks.
type ConfigView struct {
	Root            string   `json:"root"`
	BrowserPath     string   `json:"browserPath"`
	BrowserExpanded string   `json:"browserExpanded"`
	BrowserExists   bool     `json:"browserExists"`
	MaxConcurrent   int      `json:"maxConcurrent"`
	AutoCloseLRU    bool     `json:"autoCloseLRU"`
	ExtraArgs       []string `json:"extraArgs"`
	SetupComplete   bool     `json:"setupComplete"`
}

func (a *App) loadConfig() (Config, error) {
	cfg := Config{MaxConcurrent: defaultMaxConcurrent, ExtraArgs: []string{}}
	path := layout.Config(a.Root)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := jsonfile.Read(path, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = defaultMaxConcurrent
	}
	if cfg.ExtraArgs == nil {
		cfg.ExtraArgs = []string{}
	}
	return cfg, nil
}

func (a *App) saveConfig(cfg Config) error {
	if cfg.ExtraArgs == nil {
		cfg.ExtraArgs = []string{}
	}
	return jsonfile.Write(layout.Config(a.Root), cfg)
}

// ShowConfig returns the current settings.
func (a *App) ShowConfig() (ConfigView, error) {
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		view, err = a.configView()
		return err
	})
	return view, err
}

func (a *App) configView() (ConfigView, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return ConfigView{}, err
	}
	expanded := os.ExpandEnv(cfg.BrowserPath)
	exists := false
	if expanded != "" {
		if st, err := os.Stat(expanded); err == nil && !st.IsDir() {
			exists = true
		}
	}
	return ConfigView{
		Root:            a.Root,
		BrowserPath:     cfg.BrowserPath,
		BrowserExpanded: expanded,
		BrowserExists:   exists,
		MaxConcurrent:   cfg.MaxConcurrent,
		AutoCloseLRU:    cfg.AutoCloseLRU,
		ExtraArgs:       append([]string{}, cfg.ExtraArgs...),
		SetupComplete:   cfg.SetupComplete,
	}, nil
}

// SetBrowser stores the Chromium executable path.
func (a *App) SetBrowser(path string) (ConfigView, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ConfigView{}, fmt.Errorf("browser path is empty")
	}
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		cfg, err := a.loadConfig()
		if err != nil {
			return err
		}
		cfg.BrowserPath = path
		if err := a.saveConfig(cfg); err != nil {
			return err
		}
		view, err = a.configView()
		return err
	})
	return view, err
}

// SetMaxConcurrent sets the running-profile ceiling (1–500).
func (a *App) SetMaxConcurrent(n int) (ConfigView, error) {
	if n < 1 || n > maxConcurrentLimit {
		return ConfigView{}, fmt.Errorf("max-concurrent must be between 1 and %d", maxConcurrentLimit)
	}
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		cfg, err := a.loadConfig()
		if err != nil {
			return err
		}
		cfg.MaxConcurrent = n
		if err := a.saveConfig(cfg); err != nil {
			return err
		}
		view, err = a.configView()
		return err
	})
	return view, err
}

// SetSetupComplete records that the first-run wizard has finished.
func (a *App) SetSetupComplete(v bool) (ConfigView, error) {
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		cfg, err := a.loadConfig()
		if err != nil {
			return err
		}
		cfg.SetupComplete = v
		if err := a.saveConfig(cfg); err != nil {
			return err
		}
		view, err = a.configView()
		return err
	})
	return view, err
}

// SetAutoCloseLRU turns automatic closing of the least recently launched profile on or off.
func (a *App) SetAutoCloseLRU(v bool) (ConfigView, error) {
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		cfg, err := a.loadConfig()
		if err != nil {
			return err
		}
		cfg.AutoCloseLRU = v
		if err := a.saveConfig(cfg); err != nil {
			return err
		}
		view, err = a.configView()
		return err
	})
	return view, err
}

// SetExtraArgs replaces the extra Chromium arguments appended at launch.
func (a *App) SetExtraArgs(args []string) (ConfigView, error) {
	if args == nil {
		args = []string{}
	}
	var view ConfigView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		cfg, err := a.loadConfig()
		if err != nil {
			return err
		}
		cfg.ExtraArgs = append([]string{}, args...)
		if err := a.saveConfig(cfg); err != nil {
			return err
		}
		view, err = a.configView()
		return err
	})
	return view, err
}

// Registry is the on-disk index. Manifest files are the source of truth;
// the registry is rebuilt from them.
type Registry struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
	Templates []string  `json:"templates"`
	Profiles  []string  `json:"profiles"`
}

func (a *App) rebuildRegistry() error {
	templates, err := listManifestDirs(layout.Templates(a.Root))
	if err != nil {
		return err
	}
	profiles, err := listManifestDirs(layout.Profiles(a.Root))
	if err != nil {
		return err
	}
	reg := Registry{
		Version:   registryVersion,
		UpdatedAt: a.now(),
		Templates: templates,
		Profiles:  profiles,
	}
	return jsonfile.Write(layout.Registry(a.Root), reg)
}

func listManifestDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var names []string
	for _, ent := range entries {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, ent.Name(), "manifest.json")); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		names = append(names, ent.Name())
	}
	sort.Strings(names)
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (a *App) loadTemplate(name string) (manifest.Template, error) {
	var t manifest.Template
	path := layout.TemplateManifest(a.Root, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return t, fmt.Errorf("template %q not found", name)
		}
		return t, err
	}
	if err := jsonfile.Read(path, &t); err != nil {
		return t, err
	}
	t.Normalize()
	if t.Name == "" {
		t.Name = name
	}
	if t.Name != name {
		return t, fmt.Errorf("template directory %s does not match manifest name %s", name, t.Name)
	}
	return t, nil
}

func (a *App) saveTemplate(t manifest.Template) error {
	t.Normalize()
	if err := jsonfile.Write(layout.TemplateManifest(a.Root, t.Name), t); err != nil {
		return err
	}
	return a.rebuildRegistry()
}

func (a *App) loadProfile(id string) (manifest.Profile, error) {
	var p manifest.Profile
	path := layout.ProfileManifest(a.Root, id)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return p, fmt.Errorf("profile %q not found", id)
		}
		return p, err
	}
	if err := jsonfile.Read(path, &p); err != nil {
		return p, err
	}
	p.Normalize()
	if p.ID == "" {
		p.ID = id
	}
	if p.ID != id {
		return p, fmt.Errorf("profile directory %s does not match manifest id %s", id, p.ID)
	}
	return p, nil
}

func (a *App) saveProfile(p manifest.Profile) error {
	p.Normalize()
	if err := jsonfile.Write(layout.ProfileManifest(a.Root, p.ID), p); err != nil {
		return err
	}
	return a.rebuildRegistry()
}

// TemplateSummary is a template plus whether its browser is running.
type TemplateSummary struct {
	manifest.Template
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// ProfileSummary is a profile plus whether its browser is running.
type ProfileSummary struct {
	manifest.Profile
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// Inventory is every template and profile under the root.
type Inventory struct {
	Templates []TemplateSummary `json:"templates"`
	Profiles  []ProfileSummary  `json:"profiles"`
}

// List scans manifests and refreshes the registry.
func (a *App) List() (Inventory, error) {
	var inv Inventory
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		inv, err = a.listLocked()
		return err
	})
	return inv, err
}

func (a *App) listLocked() (Inventory, error) {
	running, err := a.listRunning()
	if err != nil {
		return Inventory{}, err
	}
	runKey := map[string]runningState{}
	for _, st := range running {
		runKey[st.Kind+"/"+st.ID] = st
	}
	tNames, err := listManifestDirs(layout.Templates(a.Root))
	if err != nil {
		return Inventory{}, err
	}
	pNames, err := listManifestDirs(layout.Profiles(a.Root))
	if err != nil {
		return Inventory{}, err
	}
	inv := Inventory{Templates: []TemplateSummary{}, Profiles: []ProfileSummary{}}
	for _, name := range tNames {
		t, err := a.loadTemplate(name)
		if err != nil {
			return Inventory{}, err
		}
		sum := TemplateSummary{Template: t}
		if st, ok := runKey["template/"+name]; ok {
			sum.Running = true
			sum.PID = st.PID
		}
		inv.Templates = append(inv.Templates, sum)
	}
	for _, id := range pNames {
		p, err := a.loadProfile(id)
		if err != nil {
			return Inventory{}, err
		}
		sum := ProfileSummary{Profile: p}
		if st, ok := runKey["profile/"+id]; ok {
			sum.Running = true
			sum.PID = st.PID
		}
		inv.Profiles = append(inv.Profiles, sum)
	}
	if err := a.rebuildRegistry(); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

// ProfileView is the resolved view printed by profile show.
type ProfileView struct {
	Profile     manifest.Profile       `json:"profile"`
	Resolved    []extensions.Extension `json:"resolved"`
	Running     bool                   `json:"running"`
	PID         int                    `json:"pid,omitempty"`
	UserDataDir string                 `json:"userDataDir"`
}

// ShowProfile loads a profile and resolves its extensions.
func (a *App) ShowProfile(id string) (ProfileView, error) {
	var view ProfileView
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		p, err := a.loadProfile(id)
		if err != nil {
			return err
		}
		resolved, err := a.resolve(p.Groups, p.AddExtensions, p.RemoveExtensions)
		if err != nil {
			return err
		}
		view = ProfileView{
			Profile:     p,
			Resolved:    resolved,
			UserDataDir: layout.ProfileUserData(a.Root, id),
		}
		st, ok, err := a.readRunning(layout.ProfileRunning(a.Root, id))
		if err != nil {
			return err
		}
		if ok {
			view.Running = true
			view.PID = st.PID
		}
		return nil
	})
	return view, err
}

// StatusEntry is one running browser.
type StatusEntry struct {
	Kind           string    `json:"kind"`
	ID             string    `json:"id"`
	Email          string    `json:"email,omitempty"`
	PID            int       `json:"pid"`
	RSS            uint64    `json:"rss"`
	StartedAt      time.Time `json:"startedAt"`
	LastActivityAt time.Time `json:"lastActivityAt"`
}

// Status is the running set plus the concurrent limit.
type Status struct {
	MaxConcurrent int           `json:"maxConcurrent"`
	AutoCloseLRU  bool          `json:"autoCloseLRU"`
	TotalRSS      uint64        `json:"totalRSS"`
	Running       []StatusEntry `json:"running"`
}

// Status reports running browsers and approximate RAM.
func (a *App) Status() (Status, error) {
	var st Status
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		var err error
		st, err = a.statusLocked()
		return err
	})
	return st, err
}

func (a *App) statusLocked() (Status, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return Status{}, err
	}
	running, err := a.listRunning()
	if err != nil {
		return Status{}, err
	}
	snaps, err := proc.Snapshot()
	if err != nil {
		return Status{}, fmt.Errorf("process snapshot: %w", err)
	}
	out := Status{
		MaxConcurrent: cfg.MaxConcurrent,
		AutoCloseLRU:  cfg.AutoCloseLRU,
		Running:       []StatusEntry{},
	}
	for _, r := range running {
		entry := StatusEntry{
			Kind:           r.Kind,
			ID:             r.ID,
			PID:            r.PID,
			StartedAt:      r.StartedAt,
			LastActivityAt: r.LastActivityAt,
		}
		if r.Kind == "profile" {
			if p, err := a.loadProfile(r.ID); err == nil {
				entry.Email = p.Email
			}
		}
		if r.PID > 0 {
			entry.RSS = proc.TreeRSS(snaps, r.PID)
			out.TotalRSS += entry.RSS
		}
		out.Running = append(out.Running, entry)
	}
	sort.Slice(out.Running, func(i, j int) bool {
		if out.Running[i].Kind != out.Running[j].Kind {
			return out.Running[i].Kind < out.Running[j].Kind
		}
		return out.Running[i].ID < out.Running[j].ID
	})
	return out, nil
}

// DeleteProfile removes a stopped profile and its user data.
func (a *App) DeleteProfile(id string, yes bool) error {
	if err := names.ValidID(id); err != nil {
		return err
	}
	if !yes {
		return fmt.Errorf("refusing to delete profile %s without confirmation; re-run with --yes", id)
	}
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if _, err := a.loadProfile(id); err != nil {
			return err
		}
		if st, ok, err := a.readRunning(layout.ProfileRunning(a.Root, id)); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("profile %s is running (pid %d); close it first", id, st.PID)
		}
		if err := os.RemoveAll(layout.Profile(a.Root, id)); err != nil {
			return err
		}
		return a.rebuildRegistry()
	})
}

// DeleteTemplate removes a template that no profile still references.
func (a *App) DeleteTemplate(name string, yes bool) error {
	if err := names.ValidID(name); err != nil {
		return err
	}
	if !yes {
		return fmt.Errorf("refusing to delete template %s without confirmation; re-run with --yes", name)
	}
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if _, err := a.loadTemplate(name); err != nil {
			return err
		}
		if st, ok, err := a.readRunning(layout.TemplateRunning(a.Root, name)); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("template %s is running (pid %d); close it first", name, st.PID)
		}
		profiles, err := listManifestDirs(layout.Profiles(a.Root))
		if err != nil {
			return err
		}
		var users []string
		for _, id := range profiles {
			p, err := a.loadProfile(id)
			if err != nil {
				return err
			}
			if p.Template == name {
				users = append(users, id)
			}
		}
		if len(users) > 0 {
			return fmt.Errorf("template %s is used by %s", name, strings.Join(users, ", "))
		}
		if err := os.RemoveAll(layout.Template(a.Root, name)); err != nil {
			return err
		}
		return a.rebuildRegistry()
	})
}

func (a *App) resolve(groups, add, remove []string) ([]extensions.Extension, error) {
	catalog, err := extensions.Load(a.Root)
	if err != nil {
		return nil, err
	}
	groupFile, err := extensions.LoadGroups(a.Root)
	if err != nil {
		return nil, err
	}
	resolved, err := extensions.Resolve(catalog, groupFile, groups, add, remove)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		resolved = []extensions.Extension{}
	}
	return resolved, nil
}

func (a *App) requireGroups(groupNames []string) error {
	known, err := extensions.LoadGroups(a.Root)
	if err != nil {
		return err
	}
	for _, name := range groupNames {
		if err := names.ValidGroup(name); err != nil {
			return err
		}
		if _, ok := known[name]; !ok {
			return fmt.Errorf("unknown extension group %q", name)
		}
	}
	return nil
}

func copyStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	return append([]string{}, in...)
}

func browserMissing(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("browser path is not set. Run: abrowser config set browser <path-to-chrome>\nStock Google Chrome ignores --load-extension. Use ungoogled-chromium or Chrome for Testing (see the README)")
	}
	return fmt.Errorf("browser not found at %s\nStock Google Chrome ignores --load-extension. Use ungoogled-chromium or Chrome for Testing (see the README)", os.ExpandEnv(raw))
}
