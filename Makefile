.PHONY: test build build-windows build-linux

test:
	go test ./...

build: build-linux

build-linux:
	go build -ldflags "-s -w" -o dist/abrowser ./cmd/abrowser

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/abrowser.exe ./cmd/abrowser
