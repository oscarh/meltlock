PREFIX ?= $(HOME)/.local
CONFIG ?= $(or $(XDG_CONFIG_HOME),$(HOME)/.config)

meltlock: *.go go.mod
	go build -trimpath -ldflags "-s -w" -o $@ .

test:
	go vet ./...
	go test ./...

install: meltlock
	install -Dm755 meltlock $(PREFIX)/bin/meltlock
	sed 's|@BIN@|$(PREFIX)/bin/meltlock|' meltlock.desktop | install -Dm644 /dev/stdin $(PREFIX)/share/applications/meltlock.desktop

	test -e $(CONFIG)/meltlock/effects || install -Dm644 effects.conf $(CONFIG)/meltlock/effects

.PHONY: test install
