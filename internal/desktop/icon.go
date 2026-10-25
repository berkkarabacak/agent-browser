package desktop

import _ "embed"

import "fyne.io/fyne/v2"

//go:embed icon.png
var iconPNG []byte

func appIcon() fyne.Resource {
	return fyne.NewStaticResource("agentbrowser.png", iconPNG)
}
