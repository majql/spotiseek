all: arm64 amd64
	@echo "All built"

amd64:
	@GOOS=linux GOARCH=$@ go build -o spotiseek.$@ .

arm64:
	@GOOS=darwin GOARCH=$@ go build -o spotiseek.$@ .

clean:
	rm spotiseek.*
