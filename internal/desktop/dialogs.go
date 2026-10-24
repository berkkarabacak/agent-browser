package desktop

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"abrowser/internal/app"
	"abrowser/internal/extensions"
	"abrowser/internal/manifest"
)

func isNotRunning(err error) bool {
	var nr *app.NotRunningError
	return errors.As(err, &nr)
}

func (d *desk) showClone() {
	frozen := make([]TemplateLine, 0, len(d.snap.Templates))
	names := make([]string, 0, len(d.snap.Templates))
	for _, t := range d.snap.Templates {
		if t.Frozen {
			frozen = append(frozen, t)
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		dialog.ShowInformation("No frozen template",
			"Create a template, then freeze it before cloning.\n\nabrowser template create golden --group common\nabrowser template freeze golden",
			d.win)
		return
	}
	idEnt := widget.NewEntry()
	idEnt.SetPlaceHolder("agent-001")
	emailEnt := widget.NewEntry()
	emailEnt.SetPlaceHolder("ada@example.com")
	checks := widget.NewCheckGroup(d.snap.GroupNames, nil)
	pick := widget.NewSelect(names, nil)
	pick.SetSelected(names[0])
	applyTemplate := func(name string) {
		for _, t := range frozen {
			if t.Name == name {
				checks.OnChanged = nil
				checks.SetSelected(append([]string{}, t.Groups...))
				return
			}
		}
	}
	applyTemplate(names[0])
	pick.OnChanged = applyTemplate

	form := widget.NewForm(
		widget.NewFormItem("Template", pick),
		widget.NewFormItem("ID", idEnt),
		widget.NewFormItem("Email", emailEnt),
	)
	groupBox := container.NewVBox(
		widget.NewLabel("Groups copied onto the new profile"),
		checks,
	)
	if len(d.snap.GroupNames) == 0 {
		groupBox = container.NewVBox(widget.NewLabel("No groups exist yet. The profile will be cloned with none."))
	}
	content := container.NewVBox(form, groupBox)
	dlg := dialog.NewCustomConfirm("Clone profile", "Clone", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		id := strings.TrimSpace(idEnt.Text)
		email := strings.TrimSpace(emailEnt.Text)
		template := pick.Selected
		groups := append([]string{}, checks.Selected...)
		if id == "" || email == "" || template == "" {
			d.fail(fmt.Errorf("template, id, and email are required"))
			return
		}
		d.note("Cloning " + id + "…")
		d.work(func() error {
			_, err := d.svc.Clone(app.CloneRequest{
				Template:  template,
				ID:        id,
				Email:     email,
				Groups:    groups,
				GroupsSet: true,
			})
			return err
		}, "Cloned "+id)
	}, d.win)
	dlg.Resize(fyne.NewSize(480, 420))
	dlg.Show()
}

func (d *desk) editProfile(id string) {
	d.note("Opening " + id + "…")
	go func() {
		view, err := d.svc.Show(id)
		snap, loadErr := d.svc.Load()
		fyne.Do(func() {
			if err != nil {
				d.fail(err)
				return
			}
			if loadErr == nil {
				d.apply(snap)
			}
			d.showEdit(view)
		})
	}()
}

func (d *desk) showEdit(view app.ProfileView) {
	id := view.Profile.ID
	state := view
	checks := widget.NewCheckGroup(d.snap.GroupNames, nil)
	checks.SetSelected(append([]string{}, view.Profile.Groups...))
	resolved := widget.NewLabel(resolvedText(view))
	resolved.Wrapping = fyne.TextWrapWord
	overrides := widget.NewLabel(overrideText(view.Profile))
	overrides.Wrapping = fyne.TextWrapWord

	opts := extensionRefs(d.snap.Extensions)
	sel := widget.NewSelect(opts, nil)
	if len(opts) == 0 {
		sel.PlaceHolder = "Extension store is empty"
	} else {
		sel.PlaceHolder = "Choose an extension"
	}

	refreshLabels := func(v app.ProfileView) {
		state = v
		resolved.SetText(resolvedText(v))
		overrides.SetText(overrideText(v.Profile))
	}
	reload := func() {
		go func() {
			v, err := d.svc.Show(id)
			snap, loadErr := d.svc.Load()
			fyne.Do(func() {
				if loadErr == nil {
					d.apply(snap)
				}
				if err != nil {
					d.fail(err)
					return
				}
				refreshLabels(v)
			})
		}()
	}

	addBtn := widget.NewButton("Add (+)", func() {
		d.applyExt(sel.Selected, id, "add", reload)
	})
	addBtn.Importance = widget.HighImportance
	blockBtn := widget.NewButton("Block (−)", func() {
		d.applyExt(sel.Selected, id, "remove", reload)
	})
	clearBtn := widget.NewButton("Clear override", func() {
		d.applyExt(sel.Selected, id, "reset", reload)
	})
	saveGroups := widget.NewButton("Save groups", func() {
		want := append([]string{}, checks.Selected...)
		have := append([]string{}, state.Profile.Groups...)
		d.note("Saving groups for " + id + "…")
		go func() {
			err := d.svc.SetProfileGroups(id, have, want)
			v, showErr := d.svc.Show(id)
			snap, loadErr := d.svc.Load()
			fyne.Do(func() {
				if loadErr == nil {
					d.apply(snap)
				}
				if err != nil {
					d.fail(err)
					return
				}
				if showErr == nil {
					refreshLabels(v)
					checks.SetSelected(append([]string{}, v.Profile.Groups...))
				}
				d.note("Updated groups for " + id)
			})
		}()
	})

	email := view.Profile.Email
	if email == "" {
		email = "—"
	}
	intro := widget.NewLabel(id + "   ·   " + email + "   ·   template " + orDash(view.Profile.Template))
	intro.TextStyle = fyne.TextStyle{Bold: true}
	groupLabel := widget.NewLabel("Groups")
	groupLabel.TextStyle = fyne.TextStyle{Bold: true}
	extLabel := widget.NewLabel("Extensions")
	extLabel.TextStyle = fyne.TextStyle{Bold: true}
	help := widget.NewLabel("Add pins an extra extension. Block records a −ext override (a block wins over a group). Clear override drops both. The toolbar badge is added automatically at launch.")
	help.Wrapping = fyne.TextWrapWord

	groupsBody := fyne.CanvasObject(checks)
	if len(d.snap.GroupNames) == 0 {
		groupsBody = widget.NewLabel("No groups yet. Create one on the Groups tab.")
	}
	content := container.NewVBox(
		intro,
		groupLabel,
		groupsBody,
		saveGroups,
		widget.NewSeparator(),
		extLabel,
		resolved,
		overrides,
		sel,
		container.NewHBox(addBtn, blockBtn, clearBtn),
		help,
	)
	dlg := dialog.NewCustom("Edit "+id, "Close", container.NewVScroll(content), d.win)
	dlg.Resize(fyne.NewSize(560, 520))
	dlg.Show()
}

func (d *desk) applyExt(ref, id, op string, reload func()) {
	if ref == "" {
		d.fail(fmt.Errorf("choose an extension from the store"))
		return
	}
	go func() {
		err := d.svc.ProfileExt(id, op, ref)
		snap, loadErr := d.svc.Load()
		fyne.Do(func() {
			if loadErr == nil {
				d.apply(snap)
			}
			if err != nil {
				d.fail(err)
				return
			}
			d.note(extOpNote(op, ref, id))
			if reload != nil {
				reload()
			}
		})
	}()
}

func extOpNote(op, ref, id string) string {
	switch op {
	case "add":
		return "Added " + ref + " to " + id
	case "remove":
		return "Blocked " + ref + " on " + id
	default:
		return "Cleared " + ref + " on " + id
	}
}

func resolvedText(v app.ProfileView) string {
	if len(v.Resolved) == 0 {
		return "Resolved from groups and overrides: none."
	}
	refs := make([]string, len(v.Resolved))
	for i, ext := range v.Resolved {
		refs[i] = ext.Ref()
	}
	return "Resolved: " + strings.Join(refs, ", ")
}

func overrideText(p manifest.Profile) string {
	return "Added: " + orNone(p.AddExtensions) + "\nBlocked: " + orNone(p.RemoveExtensions)
}

func extensionRefs(exts []extensions.Extension) []string {
	refs := make([]string, len(exts))
	for i, ext := range exts {
		refs[i] = ext.Ref()
	}
	return refs
}

func orNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
