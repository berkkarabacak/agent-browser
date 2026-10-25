//go:build windows

package main

import (
	"os"
	"syscall"
)

// attachConsole connects a window-subsystem exe to the parent console so
// `abrowser list` still prints in cmd.exe. A double-click has no parent
// console and opens the window with none.
func attachConsole() {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	attach := kernel.NewProc("AttachConsole")
	const attachParent = ^uintptr(0)
	r, _, _ := attach.Call(attachParent)
	if r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}
