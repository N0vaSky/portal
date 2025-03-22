.PHONY: all build clean test lint package

# Variables
BINARY_NAME=fibratus-server
GO=go
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Default target
all: clean lint test build

# Build the binary
build:
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f *.deb

# Run tests
test:
	$(GO) test -v ./...

# Run linter
lint:
	$(GO) vet ./...
	if [ -x "$(shell command -v golint)" ]; then golint ./...; else echo "golint not installed"; fi

# Build Debian package
package:
	mkdir -p dist
	dpkg-buildpackage -us -uc -b
	mv ../fibratus-portal_*.deb .

# Install development dependencies
dev-deps:
	$(GO) install golang.org/x/lint/golint@latest

# Run server in development mode
dev:
	$(GO) run ./cmd/server

# Apply database migrations
migrate:
	$(GO) run ./cmd/server -migrate

# Generate a TLS certificate for development
cert:
	mkdir -p .dev
	openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout .dev/key.pem \
		-out .dev/cert.pem \
		-subj "/CN=localhost/O=Fibratus/C=US"