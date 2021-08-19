VERSION=1.0.1
BINARY_NAME=crunchy
VERSION_BINARY_NAME=$(BINARY_NAME)-v$(VERSION)

build:
		cd cmd/crunchyroll-go && go build -o $(BINARY_NAME)
		mv cmd/crunchyroll-go/$(BINARY_NAME) .

test:
		go test -v .

release:
		cd cmd/crunchyroll-go && GOOS=linux GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_linux
		cd cmd/crunchyroll-go && GOOS=windows GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_windows.exe
		cd cmd/crunchyroll-go && GOOS=darwin GOARCH=amd64 go build -o $(VERSION_BINARY_NAME)_darwin

		mv cmd/crunchyroll-go/$(VERSION_BINARY_NAME)_* .
