//go:build windows

package desktop

import "syscall"

// initConsole detaches the console so Explorer does not leave a blank
// window behind the GUI. The binary stays a console subsystem so
// `abrowser list` still prints in cmd.exe. `abrowser gui` returns the
// prompt and keeps the window.
func initConsole() {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	free := kernel.NewProc("FreeConsole")
	_, _, _ = free.Call()
}
