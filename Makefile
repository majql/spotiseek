# Spotiseek Makefile
# Default target builds for production (linux/amd64)

.PHONY: help build build-local clean test lint fmt deps dev-setup docker-build docker-push docker-deploy run

# Default platform for production
PLATFORM := linux/amd64
LOCAL_PLATFORM := $(shell go env GOOS)/$(shell go env GOARCH)

# Docker settings
WORKER_IMAGE := majql/spotiseek-worker
WORKER_TAG := latest

help: ## Show this help message
	@echo "Spotiseek Build Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# LOCAL DEVELOPMENT (current platform - arm64 on your Mac)
# ==============================================================================

build-local: ## Build binaries for local development (current platform)
	@echo "Building for local platform: $(LOCAL_PLATFORM)"
	@mkdir -p bin/local
	go build -o bin/local/spotiseek ./cmd/spotiseek
	go build -o bin/local/worker ./cmd/worker

run: build-local ## Build and run spotiseek locally
	./bin/local/spotiseek

run-worker: build-local ## Build and run worker locally
	./bin/local/worker

run-web: build-local ## Run web interface locally
	./bin/local/spotiseek web --port 8080

# ==============================================================================
# PRODUCTION BUILD (linux/amd64 - default)
# ==============================================================================

build: ## Build binaries for production (linux/amd64)
	@echo "Building for production platform: $(PLATFORM)"
	@mkdir -p bin/amd64
	GOOS=linux GOARCH=amd64 go build -o bin/amd64/spotiseek ./cmd/spotiseek
	GOOS=linux GOARCH=amd64 go build -o bin/amd64/worker ./cmd/worker

# ==============================================================================
# DOCKER OPERATIONS
# ==============================================================================

docker-build: ## Build worker Docker image for production (linux/amd64)
	@echo "Building Docker image for $(PLATFORM)"
	docker buildx build --platform $(PLATFORM) -f Dockerfile.worker -t $(WORKER_IMAGE):$(WORKER_TAG) .

docker-build-local: ## Build worker Docker image for local platform
	@echo "Building Docker image for local platform: $(LOCAL_PLATFORM)"
	docker build -f Dockerfile.worker -t spotiseek-worker:latest .

docker-push: docker-build ## Build and push worker image to Docker Hub
	docker push $(WORKER_IMAGE):$(WORKER_TAG)

docker-deploy: docker-push ## Complete Docker deployment (build + push)
	@echo "Docker image deployed: $(WORKER_IMAGE):$(WORKER_TAG)"

# ==============================================================================
# DEVELOPMENT TOOLS
# ==============================================================================

test: ## Run all tests
	go test ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

fmt: ## Format all Go code
	go fmt ./...

deps: ## Install/update Go dependencies
	go mod download
	go mod tidy

dev-setup: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ==============================================================================
# CLEANUP
# ==============================================================================

clean: ## Clean all build artifacts
	rm -rf bin/

# ==============================================================================
# ALIASES (for backward compatibility)
# ==============================================================================

build-spotiseek: build ## Alias for build
build-worker: build ## Alias for build
build-amd64: build ## Alias for build
docker-build-worker: docker-build-local ## Alias for docker-build-local
docker-deploy-worker: docker-deploy ## Alias for docker-deploy