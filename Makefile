BINARY=hawkeye
VERSION=0.1.0

.PHONY: build clean install

build:
	go build -ldflags="-s -w" -o $(BINARY) .

install: build
	cp $(BINARY) /usr/local/bin/

clean:
	rm -f $(BINARY)

# Cross-compile for common platforms
release:
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-windows-amd64.exe .
