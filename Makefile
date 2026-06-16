# Makefile for cloud-cmdb
# Binary: cloud-cmdb  (root command with subcommands like "list-private-ips")
# Supports native build and cross-compilation for Linux and Windows (amd64)

BINARY_NAME := cloud-cmdb
MODULE      := cloud-cmdb
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

BUILD_DIR := dist

# ldflags with version information (safely ignored if the vars do not exist in main)
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -s -w"

# UPX compression (optional). Install UPX (https://upx.github.io/) to reduce binary size.
UPX      ?= upx
UPXFLAGS ?= --best --lzma

.PHONY: all build clean build-all build-linux build-windows help fmt vet test install compress build-all-upx

## Default target
all: clean build

## Build for the current platform (Linux on host)
build:
	@echo "==> Building $(BINARY_NAME) for current platform (version: $(VERSION))"
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@$(UPX) $(UPXFLAGS) $(BUILD_DIR)/$(BINARY_NAME) 2>/dev/null || true

## ============================================
## Linux
## ============================================
build-linux-amd64:
	@echo "==> Building for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@$(UPX) $(UPXFLAGS) $(BUILD_DIR)/$(BINARY_NAME) 2>/dev/null || true

## ============================================
## Windows
## ============================================
build-windows-amd64:
	@echo "==> Building for Windows amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe .
	@$(UPX) $(UPXFLAGS) $(BUILD_DIR)/$(BINARY_NAME).exe 2>/dev/null || true

## ============================================
## Useful shortcuts
## ============================================
build-linux: build-linux-amd64
	@echo "==> Linux builds completed"

build-windows: build-windows-amd64
	@echo "==> Windows builds completed"

## Full build for Linux and Windows (amd64)
build-all: clean build-linux-amd64 build-windows-amd64
	@echo ""
	@echo "==> All binaries generated in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/

## Build-all with UPX compression (requires UPX installed)
build-all-upx: clean build-linux-amd64 build-windows-amd64
	@echo ""
	@echo "==> All UPX-compressed binaries generated in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/

## Compress existing binaries in dist/ using UPX (non-fatal if UPX missing)
compress:
	@echo "==> Compressing binaries with UPX..."
	@for f in $(BUILD_DIR)/$(BINARY_NAME)*; do \
		if [ -f "$$f" ]; then \
			$(UPX) $(UPXFLAGS) "$$f" 2>/dev/null || true; \
		fi; \
	done
	@ls -lh $(BUILD_DIR)/ 2>/dev/null || true

## Cleanup
clean:
	@echo "==> Removing build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f cloud-cmdb oci-sdk-cli check-oci 2>/dev/null || true

## Formatting and verification
fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v ./...

## Install the binary to $GOPATH/bin (or $GOBIN)
install:
	go install $(LDFLAGS) .

## Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  make build                - Native build (current platform)"
	@echo "  make build-linux-amd64    - Linux x86_64 (amd64)"
	@echo "  make build-windows-amd64  - Windows x86_64 (.exe)"
	@echo "  make build-linux          - Build Linux amd64"
	@echo "  make build-windows        - Build Windows amd64"
	@echo "  make build-all            - Linux + Windows amd64 (recommended for release)"
	@echo "  make build-all-upx        - Same as build-all + UPX compression (smaller binaries)"
	@echo "  make compress             - Compress existing binaries in dist/ with UPX"
	@echo ""
	@echo "  make clean                - Remove the dist/ directory"
	@echo "  make fmt                  - Format the code"
	@echo "  make vet                  - Run go vet"
	@echo "  make test                 - Run tests"
	@echo "  make install              - Install via go install"
	@echo ""
	@echo "Binaries are generated in ./dist/"
	@echo ""
	@echo "UPX compression (optional): install 'upx' to produce significantly smaller binaries."
