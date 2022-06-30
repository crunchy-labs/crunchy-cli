VERSION=development
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

DESTDIR=
PREFIX=/usr

build:
		go build -ldflags "-X 'github.com/ByteDream/crunchy-cli/utils.Version=$(VERSION)'" -o $(BINARY_NAME) .

clean:
		rm -f $(BINARY_NAME) $(VERSION_BINARY_NAME)_*

install:
		install -Dm755 $(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/crunchy-cli
		ln -sf ./crunchy-cli $(DESTDIR)$(PREFIX)/bin/crunchy
		install -Dm644 crunchy-cli.1 $(DESTDIR)$(PREFIX)/share/man/man1/crunchy-cli.1
		install -Dm644 LICENSE $(DESTDIR)$(PREFIX)/share/licenses/crunchy-cli/LICENSE

uninstall:
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchy-cli
		rm -f $(DESTDIR)$(PREFIX)/bin/crunchy
		rm -f $(DESTDIR)$(PREFIX)/share/man/man1/crunchy-cli.1
		rm -f $(DESTDIR)$(PREFIX)/share/licenses/crunchy-cli/LICENSE

release:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchy-cli/utils.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_linux .
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchy-cli/utils.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_windows.exe .
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X 'github.com/ByteDream/crunchy-cli/utils.Version=$(VERSION)'" -o $(VERSION_BINARY_NAME)_darwin .

		strip $(VERSION_BINARY_NAME)_linux
