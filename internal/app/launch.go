package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"abrowser/internal/jsonfile"
	"abrowser/internal/layout"
	"abrowser/internal/proc"
	"abrowser/internal/profile"
)

type runningState struct {
	Kind           string    `json:"kind"`
	ID             string    `json:"id"`
	PID            int       `json:"pid"`
	StartedAt      time.Time `json:"startedAt"`
	LastActivityAt time.Time `json:"lastActivityAt"`
	Browser        string    `json:"browser"`
}

// LaunchResult describes a started browser or a dry run.
type LaunchResult struct {
	Kind        string   `json:"kind"`
	ID          string   `json:"id"`
	DryRun      bool     `json:"dryRun"`
	PID         int      `json:"pid,omitempty"`
	Browser     string   `json:"browser"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
	Extensions  []string `json:"extensions"`
	Closed      []string `json:"closedLRU,omitempty"`
	LogPath     string   `json:"logPath,omitempty"`
	UserDataDir string   `json:"userDataDir"`
}

type launchTarget struct {
	Kind        string
	ID          string
	Email       string
	Groups      []string
	Add         []string
	Remove      []string
	Timezone    string
	Locale      string
	UserDataDir string
	RunningPath string
	BadgeDir    string
	LogPath     string
	Frozen      bool
}

// LaunchProfile starts a profile, or prints the command line when dry is set.
func (a *App) LaunchProfile(id string, dry bool) (LaunchResult, error) {
	var res LaunchResult
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		p, err := a.loadProfile(id)
		if err != nil {
			return err
		}
		res, err = a.launchLocked(launchTarget{
			Kind:        "profile",
			ID:          p.ID,
			Email:       p.Email,
			Groups:      p.Groups,
			Add:         p.AddExtensions,
			Remove:      p.RemoveExtensions,
			Timezone:    p.Timezone,
			Locale:      p.Locale,
			UserDataDir: layout.ProfileUserData(a.Root, p.ID),
			RunningPath: layout.ProfileRunning(a.Root, p.ID),
			BadgeDir:    layout.ProfileBadge(a.Root, p.ID),
			LogPath:     layout.ProfileLog(a.Root, p.ID),
		}, dry)
		return err
	})
	return res, err
}

// LaunchTemplate starts a template so it can be configured. Frozen templates are refused.
func (a *App) LaunchTemplate(name string, dry bool) (LaunchResult, error) {
	var res LaunchResult
	err := a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		t, err := a.loadTemplate(name)
		if err != nil {
			return err
		}
		res, err = a.launchLocked(launchTarget{
			Kind:        "template",
			ID:          t.Name,
			Groups:      t.Groups,
			Add:         t.AddExtensions,
			Remove:      t.RemoveExtensions,
			Timezone:    t.Timezone,
			Locale:      t.Locale,
			UserDataDir: layout.TemplateUserData(a.Root, t.Name),
			RunningPath: layout.TemplateRunning(a.Root, t.Name),
			BadgeDir:    layout.TemplateBadge(a.Root, t.Name),
			LogPath:     layout.TemplateLog(a.Root, t.Name),
			Frozen:      t.Frozen,
		}, dry)
		return err
	})
	return res, err
}

// CloseProfile stops a profile browser. Closing one that is not running is success.
func (a *App) CloseProfile(id string) error {
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if _, err := a.loadProfile(id); err != nil {
			return err
		}
		return a.closeLocked("profile", id)
	})
}

// CloseTemplate stops a template browser.
func (a *App) CloseTemplate(name string) error {
	return a.withLock(func() error {
		if err := a.ensure(); err != nil {
			return err
		}
		if _, err := a.loadTemplate(name); err != nil {
			return err
		}
		return a.closeLocked("template", name)
	})
}

func (a *App) launchLocked(target launchTarget, dry bool) (LaunchResult, error) {
	if target.Kind == "template" && target.Frozen && !dry {
		return LaunchResult{}, fmt.Errorf("template %q is frozen; unfreeze it before launching again: abrowser template unfreeze %s", target.ID, target.ID)
	}
	resolved, err := a.resolve(target.Groups, target.Add, target.Remove)
	if err != nil {
		return LaunchResult{}, err
	}
	cfg, err := a.loadConfig()
	if err != nil {
		return LaunchResult{}, err
	}
	browser := os.ExpandEnv(cfg.BrowserPath)
	paths := make([]string, 0, len(resolved)+1)
	refs := make([]string, 0, len(resolved)+1)
	for _, ext := range resolved {
		paths = append(paths, ext.Path)
		refs = append(refs, ext.Ref())
	}
	badgeAbs, err := filepath.Abs(target.BadgeDir)
	if err != nil {
		return LaunchResult{}, err
	}
	paths = append(paths, badgeAbs)
	refs = append(refs, "badge")
	args, err := profile.BuildArgs(profile.ArgOptions{
		UserDataDir:    target.UserDataDir,
		ExtensionPaths: paths,
		Locale:         target.Locale,
		Extra:          cfg.ExtraArgs,
		ClassName:      profile.ClassName(target.ID),
	})
	if err != nil {
		return LaunchResult{}, err
	}
	envOverlay := profile.BuildEnv(nil, target.Timezone, target.Locale)
	result := LaunchResult{
		Kind:        target.Kind,
		ID:          target.ID,
		DryRun:      dry,
		Browser:     browser,
		Args:        args,
		Env:         envOverlay,
		Extensions:  refs,
		UserDataDir: target.UserDataDir,
		LogPath:     target.LogPath,
	}
	if dry {
		return result, nil
	}
	if err := profile.WriteBadge(target.BadgeDir, target.ID, target.Email); err != nil {
		return LaunchResult{}, err
	}
	if strings.TrimSpace(cfg.BrowserPath) == "" || browser == "" {
		return LaunchResult{}, browserMissing(cfg.BrowserPath)
	}
	if st, err := os.Stat(browser); err != nil || st.IsDir() {
		return LaunchResult{}, browserMissing(cfg.BrowserPath)
	}
	if st, ok, err := a.readRunning(target.RunningPath); err != nil {
		return LaunchResult{}, err
	} else if ok {
		return LaunchResult{}, fmt.Errorf("%s %s is already running (pid %d)", target.Kind, target.ID, st.PID)
	}
	closed, err := a.enforceLimit(target.Kind, target.ID)
	if err != nil {
		return LaunchResult{}, err
	}
	result.Closed = closed
	if err := profile.PrepareUserData(target.UserDataDir, target.ID); err != nil {
		return LaunchResult{}, err
	}
	fullEnv := profile.BuildEnv(os.Environ(), target.Timezone, target.Locale)
	cmd, err := profile.StartBrowser(profile.StartSpec{
		Browser: browser,
		Args:    args,
		Env:     fullEnv,
		LogPath: target.LogPath,
	})
	if err != nil {
		return LaunchResult{}, err
	}
	now := a.now()
	state := runningState{
		Kind:           target.Kind,
		ID:             target.ID,
		PID:            cmd.Process.Pid,
		StartedAt:      now,
		LastActivityAt: now,
		Browser:        browser,
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	if err := writeExclusiveJSON(target.RunningPath, state); err != nil {
		_ = proc.KillTree(state.PID)
		select {
		case <-waitCh:
		case <-time.After(3 * time.Second):
		}
		return LaunchResult{}, fmt.Errorf("could not record running %s %s: %w", target.Kind, target.ID, err)
	}
	select {
	case waitErr := <-waitCh:
		_ = os.Remove(target.RunningPath)
		detail := tailFile(target.LogPath, 2000)
		if waitErr == nil {
			return LaunchResult{}, fmt.Errorf("browser exited immediately (see %s)\n%s", target.LogPath, detail)
		}
		return LaunchResult{}, fmt.Errorf("browser exited immediately: %v (see %s)\n%s", waitErr, target.LogPath, detail)
	case <-time.After(a.grace()):
		result.PID = state.PID
		return result, nil
	}
}

func (a *App) enforceLimit(selfKind, selfID string) ([]string, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return nil, err
	}
	var closed []string
	for {
		running, err := a.listRunning()
		if err != nil {
			return closed, err
		}
		slots := make([]profile.Slot, 0, len(running))
		for _, st := range running {
			if st.Kind == selfKind && st.ID == selfID {
				continue
			}
			slots = append(slots, profile.Slot{
				Kind:         st.Kind,
				ID:           st.ID,
				PID:          st.PID,
				LastActivity: st.LastActivityAt,
			})
		}
		if len(slots) < cfg.MaxConcurrent {
			return closed, nil
		}
		id, err := profile.Evict(slots, cfg.MaxConcurrent, cfg.AutoCloseLRU)
		if err != nil {
			return closed, err
		}
		if err := a.closeLocked("profile", id); err != nil {
			return closed, fmt.Errorf("auto-close %s: %w", id, err)
		}
		closed = append(closed, id)
		if len(closed) > len(running)+1 {
			return closed, fmt.Errorf("could not free a profile slot")
		}
	}
}

func (a *App) closeLocked(kind, id string) error {
	path := runningPath(a.Root, kind, id)
	st, ok, err := a.readRunning(path)
	if err != nil {
		return err
	}
	if !ok {
		return &NotRunningError{Kind: kind, ID: id}
	}
	if st.PID > 0 {
		if err := proc.KillTree(st.PID); err != nil {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func runningPath(root, kind, id string) string {
	if kind == "template" {
		return layout.TemplateRunning(root, id)
	}
	return layout.ProfileRunning(root, id)
}

func (a *App) listRunning() ([]runningState, error) {
	var out []runningState
	for _, kind := range []string{"profile", "template"} {
		dir := layout.Profiles(a.Root)
		if kind == "template" {
			dir = layout.Templates(a.Root)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, ent := range entries {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			path := filepath.Join(dir, ent.Name(), "running.json")
			st, ok, err := a.readRunning(path)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if st.Kind == "" {
				st.Kind = kind
			}
			if st.ID == "" {
				st.ID = ent.Name()
			}
			out = append(out, st)
		}
	}
	if out == nil {
		out = []runningState{}
	}
	return out, nil
}

func (a *App) readRunning(path string) (runningState, bool, error) {
	var st runningState
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return st, false, nil
		}
		return st, false, err
	}
	if err := jsonfile.Read(path, &st); err != nil {
		info, statErr := os.Stat(path)
		if statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(path)
			return runningState{}, false, nil
		}
		return st, false, fmt.Errorf("reading %s: %w", path, err)
	}
	if st.PID > 0 && proc.Alive(st.PID) {
		return st, true, nil
	}
	if st.PID <= 0 {
		info, err := os.Stat(path)
		if err == nil && time.Since(info.ModTime()) < 30*time.Second {
			return st, true, nil
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return st, false, err
	}
	return runningState{}, false, nil
}

func writeExclusiveJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(buf.Bytes())
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(path)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(path)
		return cerr
	}
	return nil
}

func tailFile(path string, max int) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return strings.TrimSpace(string(b))
}
