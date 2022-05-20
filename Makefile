VERSION=development
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

DESTDIR=
PREFIX=/usr

build:
		go build -ldflags "-X 'github.com/ByteDream/crunchyroll-go/v2/cmd/crunchyroll-go/cmd.Version=$(VERSION)'" -o $(BINARY_NAME) cmd/crunchyroll-go/main.go

clean:
		rm -f $(BINARY_NAME) $(VERSION_BINARY_NAME)_*

install:
		install -Dm755 $(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		ln -sf ./crunchyroll-go $(DESTDIR)$(PREFIX)/bin/crunchy
		install -Dm644 crunchyroll-go.1 $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		install -Dm644 LICENSE $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

uninstall:
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchyroll-go
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchy
		rm -f $(DESTDIR)$(PREFIX)/share/man/man1/crunchyroll-go.1
		rm -f $(DESTDIR)$(PREFIX)/share/licenses/crunchyroll-go/LICENSE

release:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchyroll-go/v2/cmd/crunchyroll-go/cmd.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_linux cmd/crunchyroll-go/main.go
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchyroll-go/v2/cmd/crunchyroll-go/cmd.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_windows.exe cmd/crunchyroll-go/main.go
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchyroll-go/v2/cmd/crunchyroll-go/cmd.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_darwin cmd/crunchyroll-go/main.go

		strip $(VERSION_BINARY_NAME)_linux
