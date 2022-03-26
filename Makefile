VERSION=2.0.1
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

DESTDIR=
PREFIX=/usr

build:
		cd cmd/crunchyroll-go && go build -o $(BINARY_NAME)
		mv cmd/crunchyroll-go/$(BINARY_NAME) .

clean:
		rm -f $(BINARY_NAME) $(VERSION_BINARY_NAME)_*

install:
		install -Dm755 $(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		ln -sf $(DESTDIR)$(PREFIX)/bin/crunchyroll-go $(DESTDIR)$(PREFIX)/bin/crunchy
		install -Dm644 crunchyroll-go.1 $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		install -Dm644 LICENSE $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

uninstall:
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchy
		rm -f $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		rm -f $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

release:
		cd cmd/crunchyroll-go && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_linux
		cd cmd/crunchyroll-go && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_windows.exe
		cd cmd/crunchyroll-go && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_darwin

		strip cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_linux

		mv cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_* .
