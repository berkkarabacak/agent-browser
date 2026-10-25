package desktop

import (
	"context"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"abrowser/internal/update"
	"abrowser/internal/version"
)

// Run opens the desktop window and blocks until it closes.
func Run(root string) error {
	initConsole()
	svc, err := Open(root)
	if err != nil {
		return err
	}
	application := app.NewWithID("app.agentbrowser.desktop")
	application.SetIcon(appIcon())
	application.Settings().SetTheme(abTheme{})
	win := application.NewWindow("Agent Browser")
	win.SetIcon(appIcon())
	d := newDesk(svc, win)
	win.SetContent(d.build())
	win.Resize(fyne.NewSize(1100, 760))
	if snap, err := svc.Load(); err != nil {
		d.note(err.Error())
	} else {
		d.apply(snap)
	}
	win.SetCloseIntercept(func() {
		d.stop.Store(true)
		win.Close()
	})
	win.Show()
	go d.poll()
	go d.watchUpdates()
	application.Run()
	return nil
}

func (d *desk) watchUpdates() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := update.Check(ctx, version.Current, os.Getenv("ABROWSER_UPDATE_URL"))
	if err != nil || !info.Newer {
		return
	}
	text := "A newer Agent Browser is available (" + info.Latest + ")."
	if info.URL != "" {
		text += " " + info.URL
	}
	fyne.Do(func() {
		if d.update == nil || d.stop.Load() {
			return
		}
		d.update.SetText(text)
		d.updateBox.Show()
	})
}
