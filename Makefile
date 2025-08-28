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
	GOOS=linux GOARCH=amd64 go build -o bin/spotiseek-amd64 ./cmd/spotiseek

# Build the worker binary for amd64
build-worker-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/worker-amd64 ./cmd/worker

# Build worker Docker image
docker-build-worker:
	docker build -f Dockerfile.worker -t spotiseek-worker:latest .

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