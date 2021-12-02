VERSION=1.2.4
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

DESTDIR=
PREFIX=/usr

build:
		cd cmd/crunchyroll-go && go build -o $(BINARY_NAME)
		mv cmd/crunchyroll-go/$(BINARY_NAME) .

install:
		install -Dm755 $(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		cp -f $(DESTDIR)$(PREFIX)/bin/crunchyroll-go $(DESTDIR)$(PREFIX)/bin/crunchy
		install -Dm644 crunchyroll-go.1 $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		install -Dm644 LICENSE $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

uninstall:
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchy
		rm -f $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		rm -f $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

test:
		go test -v .

release:
		cd cmd/crunchyroll-go && GOOS=linux GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_linux
		cd cmd/crunchyroll-go && GOOS=windows GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_windows.exe
		cd cmd/crunchyroll-go && GOOS=darwin GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_darwin

		strip cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_linux

		mv cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_* .
