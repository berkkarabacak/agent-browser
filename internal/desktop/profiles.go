package desktop

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type profileRow struct {
	check   *widget.Check
	id      *widget.Label
	email   *widget.Label
	state   *widget.Label
	ram     *widget.Label
	meta    *widget.Label
	launch  *widget.Button
	close   *widget.Button
	edit    *widget.Button
	mute    bool
	current string
}

func (d *desk) buildProfiles() fyne.CanvasObject {
	d.search = widget.NewEntry()
	d.search.SetPlaceHolder("Filter by name, email, template, group, extension, state, or RAM")
	d.search.OnChanged = func(s string) {
		d.queryUpdate(s)
	}
	d.hint = widget.NewLabel("Loading profiles…")
	d.hint.Wrapping = fyne.TextWrapWord

	d.selectShown = widget.NewCheck("Select shown", func(on bool) {
		if d.muteSelect {
			return
		}
		for _, line := range d.filtered {
			if on {
				d.selected[line.ID] = true
			} else {
				delete(d.selected, line.ID)
			}
		}
		d.list.Refresh()
		d.updateSelLabel()
	})
	d.selLabel = widget.NewLabel("0 selected")

	clone := widget.NewButton("Clone", func() { d.showClone() })
	clone.Importance = widget.HighImportance
	launchSel := widget.NewButton("Launch selected", func() { d.launchIDs(sortedIDs(d.selected)) })
	closeSel := widget.NewButton("Close selected", func() { d.closeIDs(sortedIDs(d.selected)) })
	refresh := widget.NewButton("Refresh", func() { go d.reload() })

	d.list = widget.NewList(
		func() int { return len(d.filtered) },
		func() fyne.CanvasObject { return d.newProfileRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) { d.bindProfileRow(id, obj) },
	)

	header := container.NewBorder(nil, nil,
		widget.NewLabel(""),
		widget.NewLabel("Actions"),
		container.NewVBox(
			container.NewGridWithColumns(4,
				boldLabel("Name"),
				boldLabel("Email"),
				boldLabel("State"),
				boldLabel("RAM"),
			),
			widget.NewLabel("Template   ·   groups   ·   + added    − blocked"),
		),
	)

	toolbar := container.NewHBox(clone, launchSel, closeSel, d.selectShown, d.selLabel, refresh)
	return container.NewBorder(
		container.NewVBox(d.search, toolbar, d.hint, widget.NewSeparator(), header),
		nil, nil, nil,
		d.list,
	)
}

func boldLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

func (d *desk) newProfileRow() fyne.CanvasObject {
	row := &profileRow{
		id:    truncLabel(""),
		email: truncLabel(""),
		state: truncLabel(""),
		ram:   truncLabel(""),
		meta:  truncLabel(""),
	}
	row.id.TextStyle = fyne.TextStyle{Bold: true}
	row.check = widget.NewCheck("", func(on bool) {
		if row.mute || row.current == "" {
			return
		}
		if on {
			d.selected[row.current] = true
		} else {
			delete(d.selected, row.current)
		}
		d.updateSelLabel()
		d.syncSelectShown()
	})
	row.launch = widget.NewButton("Launch", func() {
		if row.current != "" {
			d.launchIDs([]string{row.current})
		}
	})
	row.close = widget.NewButton("Close", func() {
		if row.current != "" {
			d.closeIDs([]string{row.current})
		}
	})
	row.edit = widget.NewButton("Edit", func() {
		if row.current != "" {
			d.editProfile(row.current)
		}
	})
	body := container.NewVBox(
		container.NewGridWithColumns(4, row.id, row.email, row.state, row.ram),
		row.meta,
	)
	actions := container.NewHBox(row.launch, row.close, row.edit)
	box := container.NewPadded(container.NewBorder(nil, nil, row.check, actions, body))
	d.rows[box] = row
	return box
}

func truncLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Truncation = fyne.TextTruncateEllipsis
	l.Wrapping = fyne.TextWrapOff
	return l
}

func (d *desk) bindProfileRow(id widget.ListItemID, obj fyne.CanvasObject) {
	row := d.rows[obj]
	if row == nil || int(id) < 0 || int(id) >= len(d.filtered) {
		return
	}
	line := d.filtered[id]
	row.current = line.ID
	row.mute = true
	row.check.SetChecked(d.selected[line.ID])
	row.mute = false
	row.id.SetText(line.Title())
	row.email.SetText(line.Email)
	row.state.SetText(line.StateLabel())
	row.ram.SetText(line.RAMLabel())
	row.meta.SetText(line.MetaLine())
	if line.Running {
		row.launch.Importance = widget.MediumImportance
		row.close.Importance = widget.HighImportance
	} else {
		row.launch.Importance = widget.HighImportance
		row.close.Importance = widget.MediumImportance
	}
	row.launch.Refresh()
	row.close.Refresh()
}

func (d *desk) queryUpdate(s string) {
	d.refilter()
	_ = s
}

func (d *desk) refilter() {
	query := ""
	if d.search != nil {
		query = d.search.Text
	}
	d.filtered = FilterProfiles(d.lines, query)
	alive := map[string]bool{}
	for _, line := range d.lines {
		alive[line.ID] = true
	}
	for id := range d.selected {
		if !alive[id] {
			delete(d.selected, id)
		}
	}
	if d.list != nil {
		d.list.Refresh()
	}
	d.updateSelLabel()
	d.syncSelectShown()
	d.updateHint()
}

func (d *desk) updateHint() {
	if d.hint == nil {
		return
	}
	frozen := frozenNames(d.snap.Templates)
	switch {
	case len(d.lines) == 0 && len(frozen) == 0:
		d.hint.SetText("No profiles yet. Home → New profile asks for a name and an email.")
	case len(d.lines) == 0:
		d.hint.SetText("No profiles yet. Home → New profile, or Clone from " + strings.Join(frozen, ", ") + ".")
	case len(d.filtered) == 0:
		d.hint.SetText("No profiles match that filter.")
	case len(frozen) == 0:
		d.hint.SetText(fmt.Sprintf("%d profiles. No frozen template is available to clone.", len(d.lines)))
	default:
		d.hint.SetText(fmt.Sprintf("%d profiles. Clone from %s.", len(d.lines), strings.Join(frozen, ", ")))
	}
}

func frozenNames(templates []TemplateLine) []string {
	var names []string
	for _, t := range templates {
		if t.Frozen {
			names = append(names, t.Name)
		}
	}
	return names
}

func (d *desk) updateSelLabel() {
	if d.selLabel == nil {
		return
	}
	d.selLabel.SetText(fmt.Sprintf("%d selected", len(sortedIDs(d.selected))))
}

func (d *desk) syncSelectShown() {
	if d.selectShown == nil {
		return
	}
	all := len(d.filtered) > 0
	for _, line := range d.filtered {
		if !d.selected[line.ID] {
			all = false
			break
		}
	}
	d.muteSelect = true
	d.selectShown.SetChecked(all)
	d.muteSelect = false
}

func (d *desk) launchIDs(ids []string) {
	if len(ids) == 0 {
		d.fail(fmt.Errorf("select one or more profiles"))
		return
	}
	d.note("Launching " + strings.Join(ids, ", ") + "…")
	go func() {
		var okIDs, failed []string
		for _, id := range ids {
			if _, err := d.svc.Launch(id); err != nil {
				failed = append(failed, id+": "+err.Error())
			} else {
				okIDs = append(okIDs, id)
			}
		}
		d.finishBulk("Launched", okIDs, failed)
	}()
}

func (d *desk) closeIDs(ids []string) {
	if len(ids) == 0 {
		d.fail(fmt.Errorf("select one or more profiles"))
		return
	}
	d.note("Closing " + strings.Join(ids, ", ") + "…")
	go func() {
		var okIDs, failed []string
		for _, id := range ids {
			err := d.svc.Close(id)
			if err == nil || isNotRunning(err) {
				okIDs = append(okIDs, id)
				continue
			}
			failed = append(failed, id+": "+err.Error())
		}
		d.finishBulk("Closed", okIDs, failed)
	}()
}

func (d *desk) finishBulk(verb string, okIDs, failed []string) {
	snap, loadErr := d.svc.Load()
	fyne.Do(func() {
		if loadErr == nil {
			d.apply(snap)
		}
		if len(failed) > 0 {
			msg := strings.Join(failed, "\n")
			if len(okIDs) > 0 {
				msg = verb + " " + strings.Join(okIDs, ", ") + ".\n" + msg
			}
			d.fail(fmt.Errorf("%s", msg))
			return
		}
		d.note(verb + " " + strings.Join(okIDs, ", "))
	})
}
