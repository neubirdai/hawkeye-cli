BINARY=hawkeye
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

DOCKER_REPO=neubirdai/hawkeye-cli
LDFLAGS=-s -w -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)'

.PHONY: build clean install test lint check docker docker-push snap snap-upload release-local release-snapshot release

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

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

# Cross-compile for common platforms (local testing; use goreleaser for actual releases)
release-local:
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .

# Use goreleaser for snapshot/dry-run releases
release-snapshot:
	goreleaser release --snapshot --clean

# Tag and release a new version
release:
	@test -n "$(VERSION)" || (echo "Usage: make release VERSION=0.2.0" && exit 1)
	git tag -a "v$(VERSION)" -m "v$(VERSION)"
	git push origin "v$(VERSION)"

# Show current and next versions
version-info:
	@echo "Current version: $$(git describe --tags --abbrev=0 2>/dev/null || echo 'none')"
	@echo "Commits since:   $$(git rev-list $$(git describe --tags --abbrev=0 2>/dev/null)..HEAD --count 2>/dev/null || echo 'N/A')"

# Create and push a new patch release (v1.2.3 -> v1.2.4)
release-patch:
	@CURRENT=$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//'); \
	if [ -z "$$CURRENT" ]; then echo "No existing tags. Use: make release VERSION=0.1.0"; exit 1; fi; \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | cut -d. -f3); \
	NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	echo "Creating $$NEXT..."; \
	git tag -a $$NEXT -m "Release $$NEXT" && git push origin $$NEXT

# Create and push a new minor release (v1.2.3 -> v1.3.0)
release-minor:
	@CURRENT=$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//'); \
	if [ -z "$$CURRENT" ]; then echo "No existing tags. Use: make release VERSION=0.1.0"; exit 1; fi; \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	NEXT="v$$MAJOR.$$((MINOR + 1)).0"; \
	echo "Creating $$NEXT..."; \
	git tag -a $$NEXT -m "Release $$NEXT" && git push origin $$NEXT

docker:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_REPO):$(VERSION) -t $(DOCKER_REPO):latest --load .

docker-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_REPO):$(VERSION) -t $(DOCKER_REPO):latest --push .

snap:
	snapcraft pack

snap-upload:
	snapcraft upload $(BINARY)-cli_$(VERSION)_*.snap --release=edge
