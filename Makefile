# Go DevTrace Main Makefile

.PHONY: build test clean install example instrument-tool demo

# Build the main library
build:
	@echo "Building Go DevTrace library..."
	go build ./...

# Run all tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	go clean ./...
	cd example && make clean

# Install the library
install:
	@echo "Installing Go DevTrace..."
	go install ./...

# Build the instrumentation tool
instrument-tool:
	@echo "Building instrumentation tool..."
	cd cmd/gotrace-instrument && go build -o ../../bin/gotrace-instrument

# Run the example project
example:
	@echo "Running example project..."
	cd example && make run

# Full demonstration
demo: build instrument-tool
	@echo "🚀 Go DevTrace Full Demo"
	@echo "======================="
	cd example && make demo

# Development setup
dev-setup:
	@echo "Setting up development environment..."
	go mod tidy
	go mod download
	cd cmd/gotrace-instrument && go mod tidy
	cd example && go mod tidy
	@echo "✅ Development setup complete!"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	go vet ./...

# Run all checks
check: fmt lint test

# Generate documentation
docs:
	@echo "Generating documentation..."
	go doc -all . > GODOC.md

# Install development tools
dev-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

# Help
help:
	@echo "Available commands:"
	@echo "  build         - Build the main library"
	@echo "  test          - Run all tests"
	@echo "  bench         - Run benchmarks"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install the library"
	@echo "  instrument-tool - Build the instrumentation tool"
	@echo "  example       - Run example project"
	@echo "  demo          - Full demonstration"
	@echo "  dev-setup     - Set up development environment"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  check         - Run formatting, linting, and tests"
	@echo "  docs          - Generate documentation"
	@echo "  dev-tools     - Install development tools"
	@echo "  help          - Show this help"

# Default target
all: build test example
