# Go DevTrace Example Makefile

.PHONY: build run clean instrument demo test-instrumented

# Build the example application
build:
	@echo "Building example application..."
	go build -o bin/example main.go

# Run the example application
run:
	@echo "Running Go DevTrace example..."
	@echo "================================"
	DEVTRACE_ENABLED=true go run main.go

# Run without devtrace for comparison
run-no-trace:
	@echo "Running without DevTrace (for comparison)..."
	@echo "==========================================="
	DEVTRACE_ENABLED=false go run main.go

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -rf instrumented/

# Build the instrumentation tool
build-instrument:
	@echo "Building instrumentation tool..."
	cd ../cmd/gotrace-instrument && go build -o ../../example/bin/gotrace-instrument

# Instrument the code automatically
instrument: build-instrument
	@echo "Instrumenting code..."
	mkdir -p instrumented
	./bin/gotrace-instrument -src . -out instrumented -verbose -exclude "bin/,instrumented/"
	@echo "Instrumented code generated in ./instrumented/"
	@echo "To run instrumented version:"
	@echo "  cd instrumented && go mod tidy && go run main.go"

# Run the instrumented version
run-instrumented: instrument
	@echo "Running instrumented version..."
	@echo "=============================="
	cd instrumented && \
	go mod edit -replace github.com/hackathon/gotrace=../../ && \
	go mod tidy && \
	DEVTRACE_ENABLED=true go run main.go

# Full demonstration
demo: 
	@echo "🚀 Go DevTrace Complete Demonstration"
	@echo "====================================="
	@echo ""
	@echo "1. Running original version WITH tracing..."
	@make run
	@echo ""
	@echo "2. Running original version WITHOUT tracing..."
	@make run-no-trace
	@echo ""
	@echo "3. Running auto-instrumented version..."
	@make run-instrumented
	@echo ""
	@echo "✅ Demo complete! Check the different outputs above."

# Test the example
test:
	@echo "Testing example..."
	go test -v ./...

# Show help
help:
	@echo "Available commands:"
	@echo "  build             - Build the example application"
	@echo "  run              - Run with devtrace enabled"
	@echo "  run-no-trace     - Run with devtrace disabled"
	@echo "  instrument       - Auto-instrument the code"
	@echo "  run-instrumented - Run the auto-instrumented version"
	@echo "  demo             - Full demonstration of all features"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run tests"
	@echo "  help             - Show this help"

# Default target
all: demo
