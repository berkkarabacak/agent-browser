package desktop

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (d *desk) maybeWizard(snap Snapshot) {
	if d.wizardDecided {
		return
	}
	if snap.Config.SetupComplete || len(snap.Lines) > 0 {
		d.wizardDecided = true
		if !snap.Config.SetupComplete && len(snap.Lines) > 0 {
			go func() { _ = d.svc.FinishSetup() }()
		}
		return
	}
	d.wizardDecided = true
	d.wizardOn = true
	d.win.SetContent(d.welcomeView())
}

func (d *desk) welcomeView() fyne.CanvasObject {
	title := wizardTitle("A separate Chrome for each person")
	body := wizardBody("Agent Browser keeps each person's mail, tabs, and logins apart. Mom gets her own Chrome. Grandfather gets his own Chrome. They do not share passwords.")
	start := widget.NewButton("Get started", func() { d.beginSetup() })
	start.Importance = widget.HighImportance
	return wizardCard(title, body, start)
}

func (d *desk) beginSetup() {
	d.win.SetContent(d.progressView())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, err := d.svc.PrepareFirstRun(ctx, func(done, total int64) {
			fyne.Do(func() {
				if d.prog == nil || d.progLabel == nil {
					return
				}
				if total > 0 {
					d.prog.SetValue(float64(done) / float64(total))
					d.progLabel.SetText(fmt.Sprintf("Downloading Chrome… %d of %d MB", done>>20, total>>20))
					return
				}
				d.progLabel.SetText(fmt.Sprintf("Downloading Chrome… %d MB", done>>20))
			})
		})
		fyne.Do(func() {
			if d.stop.Load() {
				return
			}
			if err != nil {
				d.win.SetContent(d.setupErrorView(err))
				return
			}
			d.prepared = result
			d.win.SetContent(d.personView())
		})
	}()
}

func (d *desk) progressView() fyne.CanvasObject {
	title := wizardTitle("Getting Chrome ready")
	d.progLabel = wizardBody("Downloading Chrome. This can take a few minutes the first time.")
	d.prog = widget.NewProgressBar()
	return wizardCard(title, d.progLabel, d.prog)
}

func (d *desk) setupErrorView(err error) fyne.CanvasObject {
	title := wizardTitle("Chrome did not download")
	body := wizardBody(err.Error() + "\n\nCheck the internet connection, then try again.")
	retry := widget.NewButton("Try again", func() { d.beginSetup() })
	retry.Importance = widget.HighImportance
	return wizardCard(title, body, retry)
}

func (d *desk) personView() fyne.CanvasObject {
	title := wizardTitle("Who is the first person?")
	body := wizardBody("A name and an email are enough. You can add more people later.")
	note := wizardBody(fmt.Sprintf("This computer will keep up to %d Chromes open at once.", d.prepared.MaxConcurrent))
	name := widget.NewEntry()
	name.SetPlaceHolder("Mom")
	email := widget.NewEntry()
	email.SetPlaceHolder("mom@example.com")
	form := widget.NewForm(
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Email", email),
	)
	create := widget.NewButton("Create profile", func() {
		person := name.Text
		address := email.Text
		go func() {
			res, err := d.svc.CreatePerson(person, address)
			if err == nil {
				err = d.svc.FinishSetup()
			}
			snap, loadErr := d.svc.Load()
			fyne.Do(func() {
				if err != nil {
					d.note(err.Error())
					d.win.SetContent(d.personErrorView(err, person))
					return
				}
				if loadErr == nil {
					d.apply(snap)
				}
				shown := res.Profile.DisplayName
				if shown == "" {
					shown = res.Profile.ID
				}
				d.win.SetContent(d.doneView(shown))
			})
		}()
	})
	create.Importance = widget.HighImportance
	later := widget.NewButton("I'll add people later", func() {
		go func() {
			err := d.svc.FinishSetup()
			snap, loadErr := d.svc.Load()
			fyne.Do(func() {
				if err != nil {
					d.fail(err)
					return
				}
				if loadErr == nil {
					d.apply(snap)
				}
				d.wizardOn = false
				d.showMain()
				d.note("You can add someone any time with New profile.")
			})
		}()
	})
	return wizardCard(title, body, note, form, create, later)
}

func (d *desk) personErrorView(err error, person string) fyne.CanvasObject {
	title := wizardTitle("That profile was not created")
	body := wizardBody(err.Error())
	back := widget.NewButton("Try again", func() { d.win.SetContent(d.personView()) })
	back.Importance = widget.HighImportance
	_ = person
	return wizardCard(title, body, back)
}

func (d *desk) doneView(name string) fyne.CanvasObject {
	title := wizardTitle(name + " is ready")
	body := wizardBody("Open starts a Chrome that is only for " + name + ".")
	next := widget.NewButton("See my profiles", func() {
		d.wizardOn = false
		d.showMain()
		d.note(name + " is ready. Open starts their Chrome.")
	})
	next.Importance = widget.HighImportance
	return wizardCard(title, body, next)
}

func (d *desk) showMain() {
	if d.shell != nil {
		d.win.SetContent(d.shell)
	}
}

func wizardTitle(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	l.Alignment = fyne.TextAlignCenter
	l.Wrapping = fyne.TextWrapWord
	return l
}

func wizardBody(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	l.Alignment = fyne.TextAlignCenter
	return l
}

func wizardCard(objects ...fyne.CanvasObject) fyne.CanvasObject {
	box := container.NewVBox(objects...)
	return container.NewPadded(container.NewPadded(box))
}
