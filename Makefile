PREFIX ?= $(HOME)/.local

meltlock: *.go go.mod
	go build -trimpath -ldflags "-s -w" -o $@ .

test:
	go vet ./...
	go test ./...

install: meltlock
	install -Dm755 meltlock $(PREFIX)/bin/meltlock
	sed 's|@BIN@|$(PREFIX)/bin/meltlock|' meltlock.desktop | install -Dm644 /dev/stdin $(PREFIX)/share/applications/meltlock.desktop

.PHONY: test install
