.PHONY: build test integration-test test-integration e2e-test install clean help os-info

# Binary name
BINARY=aimgr

# Version information
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X github.com/dynatrace-oss/ai-config-manager/v3/pkg/version.Version=$(VERSION) \
	-X github.com/dynatrace-oss/ai-config-manager/v3/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/dynatrace-oss/ai-config-manager/v3/pkg/version.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod

# OS and Architecture Detection
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Detect Operating System
ifeq ($(UNAME_S),Linux)
    DETECTED_OS := Linux
    # Detect Linux Distro from /etc/os-release
    ifeq ($(shell test -f /etc/os-release && echo yes),yes)
        DISTRO := $(shell grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"')
    else
        DISTRO := linux
    endif
    # Set install path (XDG-compliant for Linux)
    INSTALL_PATH := ~/.local/bin
    TIMEOUT := timeout
else ifeq ($(UNAME_S),Darwin)
    DETECTED_OS := macOS
    DISTRO := 
    INSTALL_PATH := /usr/local/bin
    # Use gtimeout if available (from coreutils via brew), otherwise empty
    TIMEOUT := $(shell command -v gtimeout 2>/dev/null || echo "")
else ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
    DISTRO := 
    INSTALL_PATH := $(USERPROFILE)\AppData\Local\bin
    TIMEOUT := timeout
else
    # Fallback for other Unix systems
    DETECTED_OS := $(UNAME_S)
    DISTRO := 
    INSTALL_PATH := /usr/local/bin
    TIMEOUT := timeout
endif

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

os-info: ## Show detected OS, architecture, and installation paths
	@echo "=== System Information ==="
	@echo "Detected OS:        $(DETECTED_OS)"
ifdef DISTRO
	@echo "Linux Distro:       $(DISTRO)"
endif
	@echo "Architecture:       $(UNAME_M)"
	@echo "Install Path:       $(INSTALL_PATH)"
	@echo ""
	@echo "Build Command:      $(GOBUILD) $(LDFLAGS)"
	@echo "Timeout Command:    $(if $(TIMEOUT),$(TIMEOUT),none (no timeout available))"
	@echo ""

build: ## Build the binary
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY) -v ./cmd/aimgr

test: vet unit-test integration-test ## Run all tests (matches CI order)

unit-test: ## Run only unit tests (fast, no external dependencies)
	@echo "Running unit tests..."
	$(GOTEST) -v -short ./pkg/...

integration-test: ## Run only integration tests (slower, requires git/network)
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./pkg/...
	$(GOTEST) -v ./test/...

test-integration: ## Run integration tests (requires network, uses real GitHub repos)
	@echo "Running integration tests with real Git operations..."
	$(GOTEST) -v -tags=integration ./test/...

e2e-test: ## Run end-to-end tests (slowest, requires network, tests full CLI)
	@echo "Running E2E tests..."
ifdef TIMEOUT
	$(TIMEOUT) 10m $(GOTEST) -v -tags=e2e ./test/e2e/
else
	$(GOTEST) -v -tags=e2e ./test/e2e/
endif

install: build ## Install binary to $(INSTALL_PATH)
	mkdir -p $(INSTALL_PATH)
	cp $(BINARY) $(INSTALL_PATH)/
	@echo "Installed $(BINARY) to $(INSTALL_PATH)"
	@echo "Make sure $(INSTALL_PATH) is in your PATH"

clean: ## Clean build artifacts
	rm -f $(BINARY)
	rm -rf bin/ dist/
	$(GOCMD) clean

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

fmt: ## Format Go code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOVET) ./...

all: clean deps fmt vet test build ## Run all checks and build
