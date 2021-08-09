VERSION=1.0
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

build:
		cd cmd/crunchyroll && CGO_ENABLED=0 go build -o $(BINARY_NAME)
		mv cmd/crunchyroll/$(BINARY_NAME) .

test:
		go test -v .

release:
		cd cmd/crunchyroll && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_linux
		cd cmd/crunchyroll && GOOS=windows GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_windows.exe
		cd cmd/crunchyroll && GOOS=darwin GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_darwin

		mv cmd/crunchyroll/$(VERSION_BINARY_NAME)_* .
