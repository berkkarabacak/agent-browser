.PHONY: test build build-windows build-linux

# Fyne needs cgo. On Linux that means gcc plus the GL/X11 headers
# (libgl1-mesa-dev, libxcursor-dev, libxrandr-dev, libxinerama-dev,
# libxi-dev, libxxf86vm-dev). The Windows cross build needs mingw-w64.

test:
	go test ./...

build: build-linux

build-linux:
	CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/abrowser ./cmd/abrowser

build-windows:
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
		go build -ldflags "-s -w" -o dist/abrowser.exe ./cmd/abrowser
