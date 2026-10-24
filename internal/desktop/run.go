package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// Run opens the desktop window and blocks until it closes.
func Run(root string) error {
	initConsole()
	svc, err := Open(root)
	if err != nil {
		return err
	}
	application := app.NewWithID("app.agentbrowser.desktop")
	application.Settings().SetTheme(abTheme{})
	win := application.NewWindow("Agent Browser")
	d := newDesk(svc, win)
	win.SetContent(d.build())
	win.Resize(fyne.NewSize(1180, 760))
	win.SetCloseIntercept(func() {
		d.stop.Store(true)
		win.Close()
	})
	win.Show()
	go d.poll()
	application.Run()
	return nil
}
