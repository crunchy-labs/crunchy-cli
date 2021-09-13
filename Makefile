VERSION=1.1.0
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

build:
		cd cmd/crunchyroll-go && go build -o $(BINARY_NAME)
		mv cmd/crunchyroll-go/$(BINARY_NAME) .

install:
		cd cmd/crunchyroll-go && go build -o crunchyroll-go
		mv cmd/crunchyroll-go/crunchyroll-go /usr/bin
		ln -sf /usr/bin/crunchyroll-go /usr/bin/crunchy
		cp crunchyroll-go.1 /usr/share/man/man1

uninstall:
		unlink /usr/bin/crunchy
		rm /usr/bin/crunchyroll-go
		rm /usr/share/man/man1/crunchyroll-go.1

test:
		go test -v .

release:
		cd cmd/crunchyroll-go && GOOS=linux GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_linux
		cd cmd/crunchyroll-go && GOOS=windows GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_windows.exe
		cd cmd/crunchyroll-go && GOOS=darwin GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_darwin

		mv cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_* .
