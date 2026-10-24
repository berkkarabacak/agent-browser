package desktop

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (d *desk) buildSettings() fyne.CanvasObject {
	d.rootLabel = widget.NewLabel("—")
	d.rootLabel.Wrapping = fyne.TextWrapWord
	d.browserEntry = widget.NewEntry()
	d.browserEntry.SetPlaceHolder(`C:\Tools\chrome-win64\chrome.exe`)
	d.browserEntry.OnChanged = func(string) {
		if !d.muteSettings {
			d.settingsTouched = true
		}
	}
	d.browserState = widget.NewLabel("")
	d.browserState.Wrapping = fyne.TextWrapWord
	browse := widget.NewButton("Browse", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				d.fail(err)
				return
			}
			if r == nil {
				return
			}
			d.browserEntry.SetText(r.URI().Path())
			r.Close()
		}, d.win)
	})
	d.maxEntry = widget.NewEntry()
	d.maxEntry.SetPlaceHolder("20")
	d.maxEntry.OnChanged = func(string) {
		if !d.muteSettings {
			d.settingsTouched = true
		}
	}
	d.autoCheck = widget.NewCheck("When a launch would pass the cap, close the least recently launched profile", func(bool) {
		if !d.muteSettings {
			d.settingsTouched = true
		}
	})
	d.extraLabel = widget.NewLabel("")
	d.extraLabel.Wrapping = fyne.TextWrapWord

	save := widget.NewButton("Save settings", func() { d.saveSettings() })
	save.Importance = widget.HighImportance
	reload := widget.NewButton("Reload", func() {
		d.settingsTouched = false
		go d.reload()
	})

	form := widget.NewForm(
		widget.NewFormItem("Data root", d.rootLabel),
		widget.NewFormItem("Browser", container.NewBorder(nil, nil, nil, browse, d.browserEntry)),
		widget.NewFormItem("Browser status", d.browserState),
		widget.NewFormItem("Max concurrent", d.maxEntry),
	)
	note := widget.NewLabel("Point the browser at Chrome for Testing or ungoogled-chromium. Stock Google Chrome ignores --load-extension, so profiles would open without their extensions. Max concurrent is a ceiling from 1 to 500. On a 16 GB PC, 8–12 running profiles is a practical load.")
	note.Wrapping = fyne.TextWrapWord
	return container.NewVBox(note, form, d.autoCheck, d.extraLabel, container.NewHBox(save, reload))
}

func (d *desk) applySettings(snap Snapshot) {
	if d.rootLabel == nil {
		return
	}
	d.rootLabel.SetText(snap.Config.Root)
	if snap.Config.BrowserExists {
		d.browserState.SetText("Found " + snap.Config.BrowserExpanded)
	} else if strings.TrimSpace(snap.Config.BrowserPath) == "" {
		d.browserState.SetText("Not set.")
	} else {
		d.browserState.SetText("Not found at " + snap.Config.BrowserExpanded)
	}
	extra := "none"
	if len(snap.Config.ExtraArgs) > 0 {
		extra = strings.Join(snap.Config.ExtraArgs, " ")
	}
	d.extraLabel.SetText("Extra Chromium flags (abrowser config set extra-args): " + extra)
	if d.settingsTouched {
		return
	}
	d.muteSettings = true
	d.browserEntry.SetText(snap.Config.BrowserPath)
	d.maxEntry.SetText(strconv.Itoa(snap.Config.MaxConcurrent))
	d.autoCheck.SetChecked(snap.Config.AutoCloseLRU)
	d.muteSettings = false
}

func (d *desk) saveSettings() {
	path := strings.TrimSpace(d.browserEntry.Text)
	n, err := atoiMax(d.maxEntry.Text)
	if err != nil {
		d.fail(err)
		return
	}
	auto := d.autoCheck.Checked
	d.note("Saving settings…")
	go func() {
		view, err := d.svc.SaveSettings(path, n, auto)
		snap, loadErr := d.svc.Load()
		fyne.Do(func() {
			if err != nil {
				if loadErr == nil {
					d.apply(snap)
				}
				d.fail(err)
				return
			}
			d.settingsTouched = false
			if loadErr == nil {
				d.apply(snap)
			} else {
				d.muteSettings = true
				d.browserEntry.SetText(view.BrowserPath)
				d.maxEntry.SetText(strconv.Itoa(view.MaxConcurrent))
				d.autoCheck.SetChecked(view.AutoCloseLRU)
				d.muteSettings = false
			}
			d.note("Saved settings.")
		})
	}()
}
