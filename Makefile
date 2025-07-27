# Logs Distributor - Simple Makefile

.PHONY: help build run clean test

# Default target
help:
	@echo "🚀 Logs Distributor"
	@echo ""
	@echo "Available commands:"
	@echo "  make build         - Build the service"
	@echo "  make run           - Build and run the service"
	@echo "  make test          - Run all tests"
	@echo "  make clean         - Clean build files"
	@echo ""
	@echo "Quick start:"
	@echo "  make run"

# Build the service
build:
	@echo "Building logs-distributor..."
	@go build -o logs-distributor ./main.go
	@echo "✅ Build complete"

# Run the service
run: build
	@echo "🚀 Starting logs distributor on http://localhost:8080"
	@echo ""
	@echo "Try these commands in another terminal:"
	@echo "  curl http://localhost:8080/api/v1/health"
	@echo "  curl http://localhost:8080/api/v1/stats"
	@echo ""
	@./logs-distributor


# Run tests with verbose output
test:
	@echo "🧪 Running tests (verbose)..."
	@go mod verify
	@go test -v ./distributor/tests
	@echo "✅ Tests complete"



# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f logs-distributor
	@rm -f distributor_state.json
	@rm -f distributor_state.json.gz
	@rm -f failed_packets.json
	@echo "✅ Clean complete" 