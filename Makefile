.PHONY: all build clean test test-unit test-integration coverage lint fmt vet install release help

# Build configuration
BINARY_NAME := idrac-inventory
MAIN_PACKAGE := ./cmd/idrac-inventory
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go configuration
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w \
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GitCommit=$(GIT_COMMIT)"

# Directories
DIST_DIR := dist
COVERAGE_DIR := coverage

# Colors for output
COLOR_RESET := \033[0m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m

## help: Show this help message
help:
	@echo "iDRAC Inventory Tool - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'

## all: Build and test
all: fmt vet lint test build

## build: Build the binary
build:
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME)...$(COLOR_RESET)"
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "$(COLOR_GREEN)Build complete: ./$(BINARY_NAME)$(COLOR_RESET)"

## install: Install to $GOPATH/bin
install:
	@echo "$(COLOR_GREEN)Installing $(BINARY_NAME)...$(COLOR_RESET)"
	$(GO) install $(LDFLAGS) $(MAIN_PACKAGE)

## clean: Remove build artifacts
clean:
	@echo "$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)
	rm -rf $(COVERAGE_DIR)
	$(GO) clean -cache -testcache

## test: Run all tests
test: test-unit test-integration

## test-unit: Run unit tests
test-unit:
	@echo "$(COLOR_GREEN)Running unit tests...$(COLOR_RESET)"
	$(GO) test -v -race -short ./internal/... ./pkg/...

## test-integration: Run integration tests
test-integration:
	@echo "$(COLOR_GREEN)Running integration tests...$(COLOR_RESET)"
	$(GO) test -v -race ./tests/...

## coverage: Run tests with coverage report
coverage:
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo "$(COLOR_GREEN)Coverage report: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"

## lint: Run linter (requires golangci-lint)
lint:
	@echo "$(COLOR_GREEN)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m ./...; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not installed, skipping...$(COLOR_RESET)"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

## vet: Run go vet
vet:
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	$(GO) vet ./...

## deps: Download and tidy dependencies
deps:
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	$(GO) mod download
	$(GO) mod tidy
	$(GO) mod verify

## release: Build for multiple platforms
release: clean
	@echo "$(COLOR_GREEN)Building releases...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	
	@echo "Building linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	
	@echo "Building linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	
	@echo "Building darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	
	@echo "Building darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	
	@echo "Building windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	
	@echo "$(COLOR_GREEN)Release builds complete:$(COLOR_RESET)"
	@ls -la $(DIST_DIR)/

## docker: Build Docker image
docker:
	@echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

## run: Build and run with example config
run: build
	./$(BINARY_NAME) -config config.yaml -verbose

## run-single: Run against a single host (requires HOST, USER, PASS env vars)
run-single: build
	./$(BINARY_NAME) -host $(HOST) -user $(USER) -pass $(PASS) -verbose

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(COLOR_GREEN)All checks passed!$(COLOR_RESET)"

## version: Show version information
version:
	@echo "Version:    $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
