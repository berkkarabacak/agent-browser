package desktop

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (d *desk) buildExtensions() fyne.CanvasObject {
	d.extEmpty = widget.NewLabel("The extension store is empty. Add an unpacked folder or a .crx file.")
	d.extEmpty.Wrapping = fyne.TextWrapWord
	d.extList = widget.NewList(
		func() int { return len(d.exts) },
		func() fyne.CanvasObject {
			title := truncLabel("")
			title.TextStyle = fyne.TextStyle{Bold: true}
			sub := truncLabel("")
			return container.NewVBox(title, sub)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			if int(id) < 0 || int(id) >= len(d.exts) {
				return
			}
			item := d.exts[id]
			title := box.Objects[0].(*widget.Label)
			sub := box.Objects[1].(*widget.Label)
			title.SetText(item.Ref)
			manifest := item.Manifest
			if manifest == "" {
				manifest = item.Name
			}
			sub.SetText(manifest + "   ·   " + item.Path)
		},
	)
	d.extList.OnSelected = func(id widget.ListItemID) {
		if int(id) >= 0 && int(id) < len(d.exts) {
			d.extSelected = d.exts[id].Ref
		}
	}
	addDir := widget.NewButton("Add folder", func() { d.addExtension(false) })
	addDir.Importance = widget.HighImportance
	addCRX := widget.NewButton("Add .crx", func() { d.addExtension(true) })
	remove := widget.NewButton("Remove selected", func() { d.removeSelectedExt() })
	help := widget.NewLabel("Extensions live in the shared store as name@version. Groups and profiles point at that store; they do not each keep a copy.")
	help.Wrapping = fyne.TextWrapWord
	bar := container.NewHBox(addDir, addCRX, remove)
	return container.NewBorder(container.NewVBox(help, bar, d.extEmpty), nil, nil, nil, d.extList)
}

func (d *desk) applyExtensions(snap Snapshot) {
	items := make([]extItem, len(snap.Extensions))
	var b strings.Builder
	for i, ext := range snap.Extensions {
		items[i] = extItem{
			Ref:      ext.Ref(),
			Name:     ext.Name,
			Version:  ext.Version,
			Manifest: ext.ManifestName,
			Path:     ext.Path,
		}
		fmt.Fprintf(&b, "%s\n%s\n", items[i].Ref, items[i].Path)
	}
	sig := b.String()
	d.exts = items
	if d.groupAdd != nil {
		d.groupAdd.SetOptions(extensionRefs(snap.Extensions))
	}
	if sig == d.extSig {
		return
	}
	d.extSig = sig
	if d.extList != nil {
		d.extList.Refresh()
	}
	if d.extEmpty != nil {
		if len(items) == 0 {
			d.extEmpty.Show()
		} else {
			d.extEmpty.Hide()
		}
	}
}

func (d *desk) addExtension(crx bool) {
	nameEnt := widget.NewEntry()
	nameEnt.SetPlaceHolder("common-marker")
	verEnt := widget.NewEntry()
	verEnt.SetPlaceHolder("from manifest.json if empty")
	pathEnt := widget.NewEntry()
	if crx {
		pathEnt.SetPlaceHolder(`C:\ext\helper.crx`)
	} else {
		pathEnt.SetPlaceHolder(`C:\ext\common-marker`)
	}
	browse := widget.NewButton("Browse", func() {
		if crx {
			dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					d.fail(err)
					return
				}
				if r == nil {
					return
				}
				pathEnt.SetText(r.URI().Path())
				r.Close()
				if nameEnt.Text == "" {
					base := strings.TrimSuffix(filepath.Base(pathEnt.Text), filepath.Ext(pathEnt.Text))
					nameEnt.SetText(strings.ToLower(base))
				}
			}, d.win)
			return
		}
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				d.fail(err)
				return
			}
			if uri == nil {
				return
			}
			pathEnt.SetText(uri.Path())
			if nameEnt.Text == "" {
				nameEnt.SetText(strings.ToLower(filepath.Base(uri.Path())))
			}
		}, d.win)
	})
	kind := "unpacked folder"
	if crx {
		kind = ".crx file"
	}
	form := widget.NewForm(
		widget.NewFormItem("Name", nameEnt),
		widget.NewFormItem("Version", verEnt),
		widget.NewFormItem("Path", pathEnt),
	)
	content := container.NewVBox(
		widget.NewLabel("Copy a "+kind+" into the shared store."),
		form,
		browse,
	)
	dlg := dialog.NewCustomConfirm("Add extension", "Add", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(nameEnt.Text)
		version := strings.TrimSpace(verEnt.Text)
		path := strings.TrimSpace(pathEnt.Text)
		if name == "" || path == "" {
			d.fail(fmt.Errorf("name and path are required"))
			return
		}
		d.note("Adding " + name + "…")
		d.work(func() error {
			_, err := d.svc.AddExtension(name, version, path, crx)
			return err
		}, "Added "+name)
	}, d.win)
	dlg.Resize(fyne.NewSize(520, 320))
	dlg.Show()
}

func (d *desk) removeSelectedExt() {
	ref := d.extSelected
	if ref == "" {
		d.fail(fmt.Errorf("select an extension in the list"))
		return
	}
	dialog.ShowConfirm("Remove extension", "Remove "+ref+" from the store? Profiles that still reference it will refuse.", func(yes bool) {
		if !yes {
			return
		}
		d.work(func() error { return d.svc.RemoveExtension(ref) }, "Removed "+ref)
	}, d.win)
}
