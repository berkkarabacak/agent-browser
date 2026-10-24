package desktop

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"abrowser/internal/profile"
	"abrowser/internal/version"
)

// desk is the desktop window. Widget updates happen on the Fyne thread.
type desk struct {
	svc  *Service
	win  fyne.Window
	stop atomic.Bool

	summary   *widget.Label
	banner    *widget.Label
	bannerBox fyne.CanvasObject
	status    *widget.Label
	hint      *widget.Label

	search      *widget.Entry
	list        *widget.List
	lines       []ProfileLine
	filtered    []ProfileLine
	selected    map[string]bool
	rows        map[fyne.CanvasObject]*profileRow
	selectShown *widget.Check
	selLabel    *widget.Label
	muteSelect  bool

	exts        []extItem
	extList     *widget.List
	extSelected string
	extSig      string
	extEmpty    *widget.Label

	groupNames    []string
	groups        map[string][]string
	groupList     *widget.List
	groupSelected string
	groupDetail   *fyne.Container
	groupSig      string
	groupAdd      *widget.Select
	newGroup      *widget.Entry

	snap Snapshot

	browserEntry    *widget.Entry
	maxEntry        *widget.Entry
	autoCheck       *widget.Check
	rootLabel       *widget.Label
	extraLabel      *widget.Label
	browserState    *widget.Label
	muteSettings    bool
	settingsTouched bool
}

type extItem struct {
	Ref      string
	Name     string
	Version  string
	Manifest string
	Path     string
}

func newDesk(svc *Service, win fyne.Window) *desk {
	return &desk{
		svc:      svc,
		win:      win,
		selected: map[string]bool{},
		rows:     map[fyne.CanvasObject]*profileRow{},
		groups:   map[string][]string{},
	}
}

func (d *desk) build() fyne.CanvasObject {
	d.summary = widget.NewLabel("Running 0 / —")
	d.summary.Alignment = fyne.TextAlignTrailing
	mark := widget.NewLabel("Agent Browser")
	mark.TextStyle = fyne.TextStyle{Bold: true}
	ver := widget.NewLabel(version.Current)
	ver.Alignment = fyne.TextAlignTrailing

	d.banner = widget.NewLabel("")
	d.banner.Wrapping = fyne.TextWrapWord
	d.bannerBox = container.NewPadded(d.banner)
	d.bannerBox.Hide()

	top := container.NewVBox(
		container.NewBorder(nil, nil, mark, ver, d.summary),
		d.bannerBox,
		widget.NewSeparator(),
	)
	d.status = widget.NewLabel("Ready.")
	d.status.Truncation = fyne.TextTruncateEllipsis

	tabs := container.NewAppTabs(
		container.NewTabItem("Profiles", d.buildProfiles()),
		container.NewTabItem("Extensions", d.buildExtensions()),
		container.NewTabItem("Groups", d.buildGroups()),
		container.NewTabItem("Settings", d.buildSettings()),
	)
	tabs.SetTabLocation(container.TabLocationLeading)
	return container.NewBorder(top, container.NewPadded(d.status), nil, nil, tabs)
}

func (d *desk) poll() {
	d.reload()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if d.stop.Load() {
			return
		}
		d.reload()
	}
}

func (d *desk) reload() {
	if d.stop.Load() {
		return
	}
	snap, err := d.svc.Load()
	fyne.Do(func() {
		if d.stop.Load() {
			return
		}
		if err != nil {
			d.note(err.Error())
			return
		}
		d.apply(snap)
	})
}

func (d *desk) apply(snap Snapshot) {
	d.snap = snap
	n := len(snap.Status.Running)
	auto := ""
	if snap.Status.AutoCloseLRU {
		auto = "   ·   auto-close on"
	}
	d.summary.SetText(fmt.Sprintf("Running %d / %d   ·   RAM %s%s",
		n, snap.Status.MaxConcurrent, profile.FormatRSS(snap.Status.TotalRSS), auto))
	if snap.Config.BrowserExists {
		d.bannerBox.Hide()
	} else {
		if strings.TrimSpace(snap.Config.BrowserPath) == "" {
			d.banner.SetText("No browser path yet. On Settings, choose Chrome for Testing or ungoogled-chromium. Stock Google Chrome ignores extension loading.")
		} else {
			d.banner.SetText("Browser not found: " + snap.Config.BrowserExpanded)
		}
		d.bannerBox.Show()
	}
	d.lines = snap.Lines
	d.refilter()
	d.applyExtensions(snap)
	d.applyGroups(snap)
	d.applySettings(snap)
}

func (d *desk) note(text string) {
	d.status.SetText(text)
}

func (d *desk) fail(err error) {
	if err == nil {
		return
	}
	d.note(err.Error())
	dialog.ShowError(err, d.win)
}

func (d *desk) work(fn func() error, okMsg string) {
	go func() {
		err := fn()
		snap, loadErr := d.svc.Load()
		fyne.Do(func() {
			if loadErr == nil {
				d.apply(snap)
			}
			if err != nil {
				d.fail(err)
				return
			}
			if okMsg != "" {
				d.note(okMsg)
			}
		})
	}()
}

func sortedIDs(selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for id, on := range selected {
		if on {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func atoiMax(text string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("max concurrent must be a whole number from 1 to 500")
	}
	return n, nil
}
