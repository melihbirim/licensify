.PHONY: build run test clean docker-build docker-run help build-all release

# Variables
BINARY_NAME=licensify
VERSION?=0.1.0
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"
DOCKER_IMAGE=licensify
DOCKER_TAG=latest

# Build output directory
BUILD_DIR=dist

# Supported platforms
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) v$(VERSION) ($(GIT_COMMIT))..."
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY_NAME) main.go
	@echo "Build complete: ./$(BINARY_NAME)"

build-all: clean-dist ## Build binaries for all platforms
	@echo "Building $(BINARY_NAME) v$(VERSION) for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		output_name=$(BUILD_DIR)/$(BINARY_NAME)-$${GOOS}-$${GOARCH}; \
		if [ "$$GOOS" = "windows" ]; then \
			output_name=$${output_name}.exe; \
		fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		if [ "$$GOOS" = "darwin" ] || [ "$$GOOS" = "linux" ]; then \
			CGO_ENABLED=1 GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$output_name main.go; \
		else \
			CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$output_name main.go; \
		fi; \
		if [ $$? -eq 0 ]; then \
			echo "✓ Built: $$output_name"; \
		else \
			echo "✗ Failed: $$output_name"; \
		fi; \
	done
	@echo "Build complete! Binaries in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/

release: build-all ## Create release archives for all platforms
	@echo "Creating release archives..."
	@cd $(BUILD_DIR) && for binary in licensify-*; do \
		if [ "$$binary" != "*.tar.gz" ] && [ "$$binary" != "*.zip" ]; then \
			if echo "$$binary" | grep -q "windows"; then \
				zip "$${binary%.exe}.zip" "$$binary" ../README.md ../LICENSE; \
				echo "✓ Created: $${binary%.exe}.zip"; \
			else \
				tar czf "$$binary.tar.gz" "$$binary" ../README.md ../LICENSE; \
				echo "✓ Created: $$binary.tar.gz"; \
			fi; \
		fi; \
	done
	@echo "Release archives ready in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/*.{tar.gz,zip} 2>/dev/null || true

run: ## Run the application
	@echo "Running $(BINARY_NAME)..."
	go run main.go

dev: ## Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
	air

test: ## Run tests
	go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f *.db *.db-shm *.db-wal
	@echo "Clean complete"

clean-dist: ## Remove dist directory
	@echo "Cleaning dist directory..."
	rm -rf $(BUILD_DIR)

clean-all: clean clean-dist ## Remove all build artifacts and dist directory

deps: ## Download dependencies
	go mod download
	go mod verify

tidy: ## Tidy dependencies
	go mod tidy

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker build complete"

docker-run: ## Run Docker container
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-build-multi: ## Build multi-arch Docker image (amd64 + arm64)
	@echo "Building multi-arch Docker image..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--push .

install: build ## Install binary to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

uninstall: ## Uninstall binary from /usr/local/bin
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete"

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

format: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

all: clean deps build ## Clean, download deps, and build

checksums: ## Generate checksums for release binaries
	@echo "Generating checksums..."
	@cd $(BUILD_DIR) && shasum -a 256 licensify-* > checksums.txt
	@echo "✓ Checksums saved to $(BUILD_DIR)/checksums.txt"
	@cat $(BUILD_DIR)/checksums.txt
