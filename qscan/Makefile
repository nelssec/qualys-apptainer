VERSION := 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BINARY := qscan
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build build-linux clean deps embed test

all: deps embed build

deps:
	go mod download
	go mod tidy

embed:
	@if [ ! -f internal/embedded/qscanner-linux-amd64.gz ]; then \
		if [ -f ../qscanner-linux-amd64.gz ]; then \
			cp ../qscanner-linux-amd64.gz internal/embedded/; \
			echo "Copied qscanner-linux-amd64.gz to internal/embedded/"; \
		else \
			echo "ERROR: qscanner-linux-amd64.gz not found"; \
			echo "Please place qscanner-linux-amd64.gz in the parent directory"; \
			exit 1; \
		fi \
	fi

build: embed
	go build $(LDFLAGS) -o $(BINARY) ./cmd/qscan

build-linux: embed
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/qscan

build-all: embed
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/qscan
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/qscan
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/qscan

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-darwin-amd64 $(BINARY)-darwin-arm64

test:
	go test -v ./...

install: build
	cp $(BINARY) /usr/local/bin/

help:
	@echo "Makefile targets:"
	@echo "  all         - Download deps, embed binary, and build"
	@echo "  deps        - Download Go dependencies"
	@echo "  embed       - Copy qscanner binary for embedding"
	@echo "  build       - Build for current platform"
	@echo "  build-linux - Build for Linux amd64"
	@echo "  build-all   - Build for all platforms"
	@echo "  clean       - Remove built binaries"
	@echo "  test        - Run tests"
	@echo "  install     - Install to /usr/local/bin"
