# Rackd Makefile

# Variables
BINARY=rackd
GO=go
GOFLAGS=-v
DOCKER=docker
DOCKER_COMPOSE=docker-compose
BUN=$(shell command -v bun || echo $(HOME)/.bun/bin/bun)

# Build flags
LDFLAGS=-ldflags="-s -w"
CGO_ENABLED=0

# Directories
BUILD_DIR=./build
DOCKERFILE=./Dockerfile
WEBUI_DIR=./webui
ASSETS_DIR=./internal/ui/assets

# Git version info
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Nomad deployment
NOMAD_JOB=deployment/nomad/rackd.nomad

.PHONY: all build server cli ui ui-install ui-build ui-clean clean test docker docker-build docker-push docker-compose-up docker-compose-down nomad-run nomad-stop help

# Default target
all: build

## build: Build binary (includes UI assets)
build: ui-build binary

## binary: Build main binary
binary:
	@echo "Building binary..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

## server: Alias for binary (backward compatibility)
server: binary

## cli: Alias for binary (backward compatibility)
cli: binary

## build-linux: Build binary for Linux (for Docker)
build-linux:
	@echo "Building Linux binary..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

## install: Install binary to $GOPATH/bin
install:
	@echo "Installing binary..."
	$(GO) install $(GOFLAGS) .

## clean: Remove build artifacts
clean: ui-clean
	@echo "Cleaning..."
	$(GO) clean
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY)

## test: Run tests
test: ui-build
	@echo "Running tests..."
	@$(GO) test -v -race ./... || \
		{ echo "Tests failed"; exit 1; }

## test-short: Run short tests only
test-short:
	@echo "Running short tests..."
	$(GO) test -v -short ./...

## test-unit: Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -race ./internal/...

## test-integration: Run integration tests only
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v ./api/...

## test-coverage: Show test coverage summary
test-coverage:
	@echo "Test coverage:"
	@$(GO) test -cover ./... | grep -E '(coverage:|ok|FAIL)'

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	$(DOCKER) build -t rackd:$(VERSION) .
	$(DOCKER) tag rackd:$(VERSION) rackd:latest

## docker-push: Push Docker image to registry
docker-push:
	@echo "Pushing Docker image..."
	$(DOCKER) push rackd:$(VERSION)
	$(DOCKER) push rackd:latest

## push-tag: Create and push a git tag (usage: make push-tag TAG=v1.0.0)
push-tag:
ifndef TAG
	$(error TAG is undefined. Usage: make push-tag TAG=v1.0.0)
endif
	git tag $(TAG)
	git push origin $(TAG)

## docker-compose-up: Start services with docker-compose
docker-compose-up:
	@echo "Starting docker-compose..."
	$(DOCKER_COMPOSE) up -d
	@echo "Server available at http://localhost:8080"

## docker-compose-down: Stop docker-compose services
docker-compose-down:
	@echo "Stopping docker-compose..."
	$(DOCKER_COMPOSE) down

## docker-compose-logs: View docker-compose logs
docker-compose-logs:
	$(DOCKER_COMPOSE) logs -f

## docker-compose-ps: Show docker-compose processes
docker-compose-ps:
	$(DOCKER_COMPOSE) ps

## nomad-run: Run Nomad job
nomad-run:
	@echo "Running Nomad job..."
	nomad job run $(NOMAD_JOB)

## nomad-stop: Stop Nomad job
nomad-stop:
	@echo "Stopping Nomad job..."
	nomad job stop rackd

## nomad-restart: Restart Nomad job
nomad-restart:
	@echo "Restarting Nomad job..."
	nomad job restart rackd

## nomad-status: Show Nomad job status
nomad-status:
	@echo "Nomad job status..."
	nomad job status rackd
	nomad alloc status -job rackd

## run-server: Run server locally
run-server: binary
	@echo "Starting server..."
	$(BUILD_DIR)/$(BINARY) server

## run-cli: Run CLI locally
run-cli: binary
	@echo "Running CLI..."
	$(BUILD_DIR)/$(BINARY)

## mod-verify: Verify dependencies
mod-verify:
	$(GO) mod verify

## mod-tidy: Tidy go modules
mod-tidy:
	$(GO) mod tidy

## help: Show this help message
help:
	@echo "Rackd - Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

# Development targets
## dev: Run server with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		$(GO) run . server; \
	fi

## generate: Run go generate
generate:
	$(GO) generate ./...

## mock: Generate mocks (requires mockgen)
mock:
	@if command -v mockgen >/dev/null 2>&1; then \
		mockgen -source=internal/storage/storage.go -destination=internal/storage/mock_storage.go; \
	else \
		echo "mockgen not installed. Install with: go install github.com/golang/mock/mockgen@latest"; \
	fi

# UI build targets
## ui: Build UI assets
ui: ui-build

## ui-install: Install UI dependencies
ui-install:
	@echo "Installing UI dependencies..."
	@export PATH="$(HOME)/.bun/bin:$$PATH" && cd $(WEBUI_DIR) && bun install

## ui-build: Build UI assets
ui-build:
	@echo "Installing dependencies..."
	@export PATH="$(HOME)/.bun/bin:$$PATH" && cd $(WEBUI_DIR) && bun install
	@echo "Building UI assets..."
	@export PATH="$(HOME)/.bun/bin:$$PATH" && cd $(WEBUI_DIR) && bun run build
	@mkdir -p $(ASSETS_DIR)
	@cp $(WEBUI_DIR)/src/index.html $(ASSETS_DIR)/index.html
	@cp $(WEBUI_DIR)/dist/output.css $(ASSETS_DIR)/output.css
	@cp $(WEBUI_DIR)/dist/app.js $(ASSETS_DIR)/app.js
	@cp $(WEBUI_DIR)/src/logo.png $(ASSETS_DIR)/logo.png
	@cp $(WEBUI_DIR)/src/icon.png $(ASSETS_DIR)/icon.png
	@echo "UI assets built successfully"

## ui-clean: Remove UI build artifacts
ui-clean:
	@echo "Cleaning UI assets..."
	@cd $(WEBUI_DIR) && bun run clean 2>/dev/null || true
	@rm -rf $(ASSETS_DIR)

## ui-dev: Watch UI assets for development
ui-dev:
	@echo "Watching UI assets..."
	@export PATH="$(HOME)/.bun/bin:$$PATH" && cd $(WEBUI_DIR) && bun run watch
