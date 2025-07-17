all: arm64 amd64 tools
	@echo "All built"

amd64:
	@GOOS=linux GOARCH=$@ go build -o spotiseek.$@ ./cmd/spotiseek

arm64:
	@GOOS=darwin GOARCH=$@ go build -o spotiseek.$@ ./cmd/spotiseek

tools:
	@go build -o clear-searches ./cmd/clear-searches

clean:
	rm -f spotiseek.* clear-searches
