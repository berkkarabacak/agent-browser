package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"abrowser/internal/app"
	"abrowser/internal/browserfetch"
	"abrowser/internal/extensions"
	"abrowser/internal/memory"
	"abrowser/internal/names"
	"abrowser/internal/webstore"
)

// FirstRun is what the wizard prepared.
type FirstRun struct {
	MaxConcurrent int
	BrowserPath   string
	Downloaded    bool
	Version       string
}

// PrepareFirstRun creates the shared list and the golden starting point,
// picks a concurrent cap from free RAM, and makes sure Chrome is available.
func (s *Service) PrepareFirstRun(ctx context.Context, progress func(done, total int64)) (FirstRun, error) {
	n := 4
	if info, err := memory.SystemMemory(); err == nil {
		n = memory.SuggestMaxConcurrent(info.Total, info.Free)
	}
	s.mu.Lock()
	if _, err := s.core.SetMaxConcurrent(n); err != nil {
		s.mu.Unlock()
		return FirstRun{}, err
	}
	if err := s.ensureStarterLocked(); err != nil {
		s.mu.Unlock()
		return FirstRun{}, err
	}
	cfg, err := s.core.ShowConfig()
	if err != nil {
		s.mu.Unlock()
		return FirstRun{}, err
	}
	result := FirstRun{MaxConcurrent: n, BrowserPath: cfg.BrowserExpanded}
	if cfg.BrowserExists {
		s.mu.Unlock()
		return result, nil
	}
	if env := strings.TrimSpace(os.Getenv("ABROWSER_BROWSER")); env != "" {
		if st, statErr := os.Stat(env); statErr == nil && !st.IsDir() {
			view, err := s.core.SetBrowser(env)
			s.mu.Unlock()
			if err != nil {
				return FirstRun{}, err
			}
			result.BrowserPath = view.BrowserExpanded
			return result, nil
		}
	}
	root := s.core.Root
	s.mu.Unlock()

	fetched, err := browserfetch.Fetch(ctx, browserfetch.Options{
		DestDir:     filepath.Join(root, "browser"),
		Progress:    progress,
		ManifestURL: strings.TrimSpace(os.Getenv("ABROWSER_CFT_MANIFEST")),
	})
	if err != nil {
		return FirstRun{}, fmt.Errorf("could not download Chrome: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.core.SetBrowser(fetched.Executable)
	if err != nil {
		return FirstRun{}, err
	}
	result.BrowserPath = view.BrowserExpanded
	result.Downloaded = true
	result.Version = fetched.Version
	return result, nil
}

// FinishSetup hides the first-run wizard on later opens.
func (s *Service) FinishSetup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.core.SetSetupComplete(true)
	return err
}

// CreatePerson clones the golden starting point using a name and an email.
func (s *Service) CreatePerson(name, email string) (app.CloneResult, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return app.CloneResult{}, fmt.Errorf("type a name")
	}
	runes := []rune(name)
	if len(runes) > 80 {
		return app.CloneResult{}, fmt.Errorf("use a shorter name")
	}
	slug, err := names.SlugID(name)
	if err != nil {
		return app.CloneResult{}, err
	}
	if err := names.ValidEmail(email); err != nil {
		return app.CloneResult{}, fmt.Errorf("email should look like mom@example.com")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStarterLocked(); err != nil {
		return app.CloneResult{}, err
	}
	inv, err := s.core.List()
	if err != nil {
		return app.CloneResult{}, err
	}
	taken := map[string]bool{}
	for _, p := range inv.Profiles {
		taken[p.ID] = true
	}
	id, err := names.UniqueID(slug, func(candidate string) bool { return taken[candidate] })
	if err != nil {
		return app.CloneResult{}, err
	}
	return s.core.Clone(app.CloneRequest{
		Template:    "golden",
		ID:          id,
		Email:       email,
		DisplayName: name,
	})
}

// AddWebStore downloads a Chrome Web Store extension and pins it on the
// shared list ("common") or on one profile.
func (s *Service) AddWebStore(ctx context.Context, link, target string) (extensions.Extension, error) {
	id, err := webstore.ParseID(link)
	if err != nil {
		return extensions.Extension{}, err
	}
	crx, err := os.CreateTemp("", "abrowser-*.crx")
	if err != nil {
		return extensions.Extension{}, err
	}
	crxName := crx.Name()
	crx.Close()
	defer os.Remove(crxName)
	if err := webstore.Download(ctx, id, crxName, nil); err != nil {
		return extensions.Extension{}, err
	}
	unpacked, err := os.MkdirTemp("", "abrowser-ext-*")
	if err != nil {
		return extensions.Extension{}, err
	}
	defer os.RemoveAll(unpacked)
	if err := extensions.ExtractCRX(crxName, unpacked); err != nil {
		return extensions.Extension{}, fmt.Errorf("that download was not a Chrome extension: %w", err)
	}
	mf, err := webstore.ReadManifest(filepath.Join(unpacked, "manifest.json"))
	if err != nil {
		return extensions.Extension{}, err
	}
	storeName := webstore.ExtensionSlug(mf.Name, id)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureCommonLocked(); err != nil {
		return extensions.Extension{}, err
	}
	ext, err := s.core.AddExtensionFromDir(storeName, mf.Version, unpacked)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		list, lerr := s.core.Extensions()
		if lerr != nil {
			return extensions.Extension{}, lerr
		}
		found := false
		for _, item := range list {
			if item.Name == storeName && item.Version == mf.Version {
				ext = item
				found = true
				break
			}
		}
		if !found {
			return extensions.Extension{}, err
		}
	} else if err != nil {
		return extensions.Extension{}, err
	}
	target = strings.TrimSpace(target)
	if target == "" || target == "common" {
		if _, err := s.core.GroupAdd("common", ext.Ref()); err != nil {
			return extensions.Extension{}, err
		}
		return ext, nil
	}
	if _, err := s.core.ProfileExt(target, "add", ext.Ref()); err != nil {
		return extensions.Extension{}, err
	}
	return ext, nil
}

func (s *Service) ensureCommonLocked() error {
	if err := s.core.CreateGroup("common"); err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

func (s *Service) ensureStarterLocked() error {
	if err := s.ensureCommonLocked(); err != nil {
		return err
	}
	inv, err := s.core.List()
	if err != nil {
		return err
	}
	found := false
	frozen := false
	for _, t := range inv.Templates {
		if t.Name == "golden" {
			found = true
			frozen = t.Frozen
			break
		}
	}
	if !found {
		if _, err := s.core.CreateTemplate(app.TemplateCreate{
			Name:   "golden",
			Locale: "en-US",
			Groups: []string{"common"},
			Notes:  "Starting point created by Agent Browser.",
		}); err != nil {
			return err
		}
	}
	if !frozen {
		if _, err := s.core.FreezeTemplate("golden"); err != nil {
			return err
		}
	}
	return nil
}
