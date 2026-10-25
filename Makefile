.PHONY: test build build-windows build-linux installer

# Fyne needs cgo. On Linux that means gcc plus the GL/X11 headers
# (libgl1-mesa-dev, libxcursor-dev, libxrandr-dev, libxinerama-dev,
# libxi-dev, libxxf86vm-dev). The Windows build needs mingw-w64.
# The installer also needs NSIS (makensis).

VERSION := $(shell sed -n 's/^const Current = "\(.*\)"/\1/p' internal/version/version.go)

test:
	go test ./...

build: build-linux

build-linux:
	CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/abrowser ./cmd/abrowser

cmd/abrowser/rsrc_windows_amd64.syso: assets/agentbrowser.ico cmd/abrowser/windows.rc
	x86_64-w64-mingw32-windres -O coff -i cmd/abrowser/windows.rc -o cmd/abrowser/rsrc_windows_amd64.syso

# The Windows exe is a GUI program: double-click and the shortcuts open the
# window with no console. From cmd.exe it attaches to that console so
# commands such as "abrowser list" still print.
build-windows: cmd/abrowser/rsrc_windows_amd64.syso
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
		go build -ldflags "-s -w -H windowsgui" -o dist/abrowser.exe ./cmd/abrowser

installer: build-windows
	mkdir -p dist
	makensis -DVERSION=$(VERSION) -DROOT=$(CURDIR) installer/agentbrowser.nsi
