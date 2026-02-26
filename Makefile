BINARY=hawkeye

.PHONY: build clean install test lint check release-snapshot release

build:
	go build -ldflags="-s -w" -o $(BINARY) .

install: build
	cp $(BINARY) /usr/local/bin/

clean:
	rm -f $(BINARY)
	rm -rf dist/

test:
	go test ./... -count=1 -timeout 30s

lint:
	golangci-lint run ./...

check: lint test

release-snapshot:
	goreleaser release --snapshot --clean

release:
	@test -n "$(VERSION)" || (echo "Usage: make release VERSION=0.2.0" && exit 1)
	git tag -a "v$(VERSION)" -m "v$(VERSION)"
	git push origin "v$(VERSION)"
	goreleaser release --clean
