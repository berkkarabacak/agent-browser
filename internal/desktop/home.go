package desktop

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type homeRow struct {
	name   *widget.Label
	email  *widget.Label
	action *widget.Button
	id     string
}

func (d *desk) buildHome() fyne.CanvasObject {
	intro := widget.NewLabel("Each person has their own Chrome.")
	intro.Wrapping = fyne.TextWrapWord
	d.homeEmpty = widget.NewLabel("No people yet. New profile asks for a name and an email.")
	d.homeEmpty.Wrapping = fyne.TextWrapWord

	newProfile := widget.NewButton("New profile", func() { d.showNewPerson() })
	newProfile.Importance = widget.HighImportance
	addExt := widget.NewButton("Add an extension", func() { d.showAddExtension() })
	actions := container.NewGridWithColumns(2, newProfile, addExt)

	d.homeList = widget.NewList(
		func() int { return len(d.homeLines) },
		func() fyne.CanvasObject { return d.newHomeRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) { d.bindHomeRow(id, obj) },
	)
	return container.NewBorder(
		container.NewVBox(intro, actions, d.homeEmpty, widget.NewSeparator()),
		nil, nil, nil,
		d.homeList,
	)
}

func (d *desk) newHomeRow() fyne.CanvasObject {
	row := &homeRow{
		name:  widget.NewLabel("Name"),
		email: widget.NewLabel("email"),
	}
	row.name.TextStyle = fyne.TextStyle{Bold: true}
	row.action = widget.NewButton("Open", func() {
		if row.id == "" {
			return
		}
		running := false
		for _, line := range d.homeLines {
			if line.ID == row.id {
				running = line.Running
				break
			}
		}
		if running {
			d.homeClose(row.id)
			return
		}
		d.homeOpen(row.id)
	})
	row.action.Importance = widget.HighImportance
	box := container.NewPadded(container.NewVBox(row.name, row.email, row.action))
	d.homeRows[box] = row
	return box
}

func (d *desk) bindHomeRow(id widget.ListItemID, obj fyne.CanvasObject) {
	row := d.homeRows[obj]
	if row == nil || int(id) < 0 || int(id) >= len(d.homeLines) {
		return
	}
	line := d.homeLines[id]
	row.id = line.ID
	row.name.SetText(line.Title())
	row.email.SetText(line.Email)
	if line.Running {
		row.action.SetText("Close")
	} else {
		row.action.SetText("Open")
	}
}

func (d *desk) refreshHome() {
	if d.homeList == nil {
		return
	}
	d.homeLines = append([]ProfileLine(nil), d.lines...)
	if d.homeEmpty != nil {
		if len(d.homeLines) == 0 {
			d.homeEmpty.SetText("No people yet. New profile asks for a name and an email.")
		} else {
			d.homeEmpty.SetText("Open starts that person's Chrome. Close stops it.")
		}
	}
	d.homeList.Refresh()
}

func (d *desk) homeOpen(id string) {
	title := d.titleFor(id)
	d.work(func() error {
		_, err := d.svc.Launch(id)
		return err
	}, title+" — Chrome is opening.")
}

func (d *desk) homeClose(id string) {
	title := d.titleFor(id)
	d.work(func() error {
		return d.svc.Close(id)
	}, title+" — Chrome is closing.")
}

func (d *desk) titleFor(id string) string {
	for _, line := range d.lines {
		if line.ID == id {
			return line.Title()
		}
	}
	return id
}

func (d *desk) showNewPerson() {
	name := widget.NewEntry()
	name.SetPlaceHolder("Mom")
	email := widget.NewEntry()
	email.SetPlaceHolder("mom@example.com")
	form := widget.NewForm(
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Email", email),
	)
	help := widget.NewLabel("A name and an email are enough. Each person gets a separate Chrome.")
	help.Wrapping = fyne.TextWrapWord
	body := container.NewVBox(help, form)
	var dlg dialog.Dialog
	dlg = dialog.NewCustomConfirm("New profile", "Create profile", "Cancel", body, func(ok bool) {
		if !ok {
			return
		}
		person := name.Text
		d.work(func() error {
			_, err := d.svc.CreatePerson(person, email.Text)
			return err
		}, "Profile created. Open starts their Chrome.")
	}, d.win)
	dlg.Resize(fyne.NewSize(520, 320))
	dlg.Show()
}

func (d *desk) showAddExtension() {
	link := widget.NewEntry()
	link.SetPlaceHolder("https://chromewebstore.google.com/detail/…")
	labels := []string{"Everyone"}
	targets := map[string]string{"Everyone": "common"}
	for _, line := range d.lines {
		label := line.Title()
		if line.DisplayName != "" && line.DisplayName != line.ID {
			label = fmt.Sprintf("%s (%s)", line.Title(), line.ID)
		}
		if _, exists := targets[label]; exists {
			label = label + " " + line.ID
		}
		labels = append(labels, label)
		targets[label] = line.ID
	}
	who := widget.NewSelect(labels, nil)
	who.SetSelected("Everyone")
	help := widget.NewLabel("Paste a Chrome Web Store link, or the long id from that page. Everyone shares it. One person keeps it only on their Chrome.")
	help.Wrapping = fyne.TextWrapWord
	body := container.NewVBox(help, link, widget.NewLabel("Add it for"), who)
	dlg := dialog.NewCustomConfirm("Add an extension", "Add extension", "Cancel", body, func(ok bool) {
		if !ok {
			return
		}
		target := targets[who.Selected]
		raw := link.Text
		d.note("Downloading the extension…")
		go func() {
			ext, err := d.svc.AddWebStore(context.Background(), raw, target)
			snap, loadErr := d.svc.Load()
			fyne.Do(func() {
				if loadErr == nil {
					d.apply(snap)
				}
				if err != nil {
					d.fail(err)
					return
				}
				label := ext.ManifestName
				if label == "" {
					label = ext.Ref()
				}
				whoFor := "everyone"
				if target != "common" {
					whoFor = d.titleFor(target)
				}
				d.note(label + " was added for " + whoFor + ".")
			})
		}()
	}, d.win)
	dlg.Resize(fyne.NewSize(560, 340))
	dlg.Show()
}
