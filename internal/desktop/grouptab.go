package desktop

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (d *desk) buildGroups() fyne.CanvasObject {
	d.newGroup = widget.NewEntry()
	d.newGroup.SetPlaceHolder("common")
	create := widget.NewButton("Create group", func() {
		name := strings.TrimSpace(d.newGroup.Text)
		if name == "" {
			d.fail(fmt.Errorf("enter a group name"))
			return
		}
		d.work(func() error { return d.svc.CreateGroup(name) }, "Created group "+name)
	})
	create.Importance = widget.HighImportance

	d.groupList = widget.NewList(
		func() int { return len(d.groupNames) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.TextStyle = fyne.TextStyle{Bold: true}
			return l
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if int(id) >= 0 && int(id) < len(d.groupNames) {
				name := d.groupNames[id]
				n := len(d.groups[name])
				label.SetText(fmt.Sprintf("%s  (%d)", name, n))
			}
		},
	)
	d.groupList.OnSelected = func(id widget.ListItemID) {
		if int(id) < 0 || int(id) >= len(d.groupNames) {
			return
		}
		d.groupSelected = d.groupNames[id]
		d.groupSig = ""
		d.renderGroupDetail()
	}

	d.groupAdd = widget.NewSelect(nil, nil)
	d.groupAdd.PlaceHolder = "Extension to pin"
	add := widget.NewButton("Add to group", func() {
		if d.groupSelected == "" {
			d.fail(fmt.Errorf("select a group"))
			return
		}
		ref := d.groupAdd.Selected
		if ref == "" {
			d.fail(fmt.Errorf("choose an extension"))
			return
		}
		group := d.groupSelected
		d.work(func() error { return d.svc.GroupAdd(group, ref) }, "Pinned "+ref+" in "+group)
	})

	d.groupDetail = container.NewVBox(widget.NewLabel("Select a group."))
	left := container.NewBorder(
		widget.NewLabel("Groups"),
		container.NewVBox(d.newGroup, create),
		nil, nil,
		d.groupList,
	)
	right := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Members are pinned as name@version. Adding a newer version replaces the older one."),
			container.NewBorder(nil, nil, nil, add, d.groupAdd),
		),
		nil, nil, nil,
		container.NewVScroll(d.groupDetail),
	)
	split := container.NewHSplit(left, right)
	split.SetOffset(0.34)
	return split
}

func (d *desk) applyGroups(snap Snapshot) {
	d.groups = snap.Groups
	d.groupNames = snap.GroupNames
	if d.groupList != nil {
		d.groupList.Refresh()
	}
	found := -1
	for i, name := range d.groupNames {
		if name == d.groupSelected {
			found = i
			break
		}
	}
	if d.groupSelected == "" && len(d.groupNames) > 0 {
		d.groupSelected = d.groupNames[0]
		found = 0
	}
	if found == -1 {
		d.groupSelected = ""
		d.groupSig = ""
		if d.groupList != nil {
			d.groupList.UnselectAll()
		}
	} else if d.groupList != nil {
		// Select is a no-op when this row is already selected, so the
		// poll does not rebuild the member list on every tick.
		d.groupList.Select(found)
	}
	d.renderGroupDetail()
}

func (d *desk) renderGroupDetail() {
	if d.groupDetail == nil {
		return
	}
	name := d.groupSelected
	if name == "" {
		d.groupDetail.Objects = []fyne.CanvasObject{
			widget.NewLabel("No group yet. Create one, for example “common”, then pin an extension into it."),
		}
		d.groupDetail.Refresh()
		return
	}
	members := d.groups[name]
	sig := name + "\n" + strings.Join(members, "\n")
	if sig == d.groupSig {
		return
	}
	d.groupSig = sig
	title := widget.NewLabel(name)
	title.TextStyle = fyne.TextStyle{Bold: true}
	objs := []fyne.CanvasObject{title}
	if len(members) == 0 {
		objs = append(objs, widget.NewLabel("This group has no extensions yet."))
	}
	for _, member := range members {
		member := member
		label := truncLabel(member)
		btn := widget.NewButton("Remove", func() {
			group := name
			d.work(func() error { return d.svc.GroupRemove(group, member) }, "Removed "+member+" from "+group)
		})
		objs = append(objs, container.NewBorder(nil, nil, nil, btn, label))
	}
	d.groupDetail.Objects = objs
	d.groupDetail.Refresh()
}
