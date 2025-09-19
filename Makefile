.PHONY: build build-worker build-spotiseek build-amd64 build-spotiseek-amd64 build-worker-amd64 clean test docker-build-worker

# Build both binaries
build: build-spotiseek build-worker

# Build the main spotiseek binary
build-spotiseek:
	go build -o bin/spotiseek ./cmd/spotiseek

# Build the worker binary
build-worker:
	go build -o bin/worker ./cmd/worker

# Build both binaries for amd64
build-amd64: build-spotiseek-amd64 build-worker-amd64

# Build the main spotiseek binary for amd64
build-spotiseek-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/amd64/spotiseek ./cmd/spotiseek

# Build the worker binary for amd64
build-worker-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/amd64/worker ./cmd/worker

# Build worker Docker image locally
docker-build-worker:
	docker build -f Dockerfile.worker -t spotiseek-worker:latest .

# Build and tag worker image for Docker Hub
docker-build-worker-hub:
	docker build -f Dockerfile.worker -t majql/spotiseek-worker:latest .

# Push worker image to Docker Hub
docker-push-worker:
	docker push majql/spotiseek-worker:latest

# Build and push worker image to Docker Hub
docker-deploy-worker: docker-build-worker-hub docker-push-worker

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Build and run spotiseek locally
run-spotiseek: build-spotiseek
	./bin/spotiseek

# Build and run worker locally
run-worker: build-worker
	./bin/worker

# Run web interface (requires Spotify credentials)
run-web: build-spotiseek
	./bin/spotiseek web --port 8080

# Development setup
dev-setup:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
