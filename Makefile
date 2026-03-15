BINARY    := gh-history
MODULE    := github.com/ivuorinen/gh-history
GOFLAGS   := -trimpath
LDFLAGS   := -s -w

# Build for the current platform
.PHONY: build
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Run all tests
.PHONY: test
test:
	go test ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	go test -v ./...

# Run tests with race detector
.PHONY: test-race
test-race:
	go test -race ./...

# Run tests with coverage summary
.PHONY: test-cov
test-cov:
	go test -cover ./...

# Vet + staticcheck (install staticcheck with: go install honnef.co/go/tools/cmd/staticcheck@latest)
.PHONY: lint
lint:
	go vet ./...
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

# Cross-compile for common platforms
.PHONY: build-all
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux   GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .

build-linux-arm64:
	GOOS=linux   GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .

build-darwin-amd64:
	GOOS=darwin  GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .

build-darwin-arm64:
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .

# Install locally as a gh extension
# Assumes the repo is already cloned and you're running from inside it
.PHONY: install
install: build
	gh extension install .

# Remove locally installed extension
.PHONY: uninstall
uninstall:
	gh extension remove history

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf dist/

.PHONY: dist
dist:
	mkdir -p dist

# Create a calver release (must be on clean main branch)
.PHONY: release
release:
	@command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI not installed"; exit 1; }
	@gh extension list | grep -q calver || gh extension install ivuorinen/gh-calver
	@if ! git diff --quiet || ! git diff --cached --quiet; then \
		echo "Error: working tree is not clean"; exit 1; \
	fi
	@if [ "$$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then \
		echo "Error: not on main branch"; exit 1; \
	fi
	$(eval TAG := $(shell gh calver next))
	@echo "Releasing $(TAG)..."
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "Tag $(TAG) pushed. GitHub Actions will build and sign the release."

.PHONY: all
all: lint test build
