# Set shell explicitly
SHELL=/usr/bin/bash

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary name
BINARY_NAME=app
TESTCLIENT_NAME=testclient
BINARY_DIR=bin

# Main package path
MAIN_PACKAGE=./cmd/app
TESTCLIENT_PACKAGE=./cmd/testclient

.PHONY: all build run test test-coverage clean deps fmt lint proto swagger rename-module help test-endpoints test-streams test-grpc bench-grpc run-grpc

# Default target
all: clean deps fmt swagger test build


# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Run ConnectRPC endpoint integration tests (requires a running server at localhost:50051)
test-endpoints:
	@echo "Running endpoint tests against $(GRPC_ADDR)..."
	$(BINARY_DIR)/$(TESTCLIENT_NAME) -mode endpoints -addr $(GRPC_ADDR) -base-dir . -timeout $(ENDPOINTS_TIMEOUT)

# Run ConnectRPC stream load test (requires a running server at localhost:50051)
test-streams:
	@echo "Running stream load test against $(GRPC_ADDR) (step=$(STEP_SIZE), max=$(MAX_STREAMS), fanout=$(FANOUT))..."
	$(BINARY_DIR)/$(TESTCLIENT_NAME) -mode streams -addr $(GRPC_ADDR) -base-dir . \
		-max-streams $(MAX_STREAMS) -step $(STEP_SIZE) -fanout=$(FANOUT) -timeout $(STREAMS_TIMEOUT)

# Run throughput benchmark (requires a running server at localhost:50051)
bench-grpc: build
	@echo "Running throughput benchmark against $(GRPC_ADDR)..."
	$(BINARY_DIR)/$(TESTCLIENT_NAME) -mode bench -addr $(GRPC_ADDR) -base-dir . \
		-workload $(BENCH_WORKLOAD) -workers $(BENCH_WORKERS) \
		-duration $(BENCH_DURATION) -warmup $(BENCH_WARMUP)

# Run both endpoint tests and stream load test
test-grpc: test-endpoints test-streams

GRPC_ADDR ?= localhost:50051
MAX_STREAMS ?= 6000
STEP_SIZE ?= 100
FANOUT ?= true
ENDPOINTS_TIMEOUT ?= 60s
STREAMS_TIMEOUT ?= 120s
BENCH_WORKLOAD ?= mixed
BENCH_WORKERS  ?= 10
BENCH_DURATION ?= 10s
BENCH_WARMUP   ?= 2s

# Build the application
build:
	@echo "Building..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	$(GOBUILD) -o $(BINARY_DIR)/$(TESTCLIENT_NAME) $(TESTCLIENT_PACKAGE)
	@echo "Build complete: $(BINARY_DIR)/$(BINARY_NAME), $(BINARY_DIR)/$(TESTCLIENT_NAME)"

# Run the application
run: build
	@echo "Running..."
	./$(BINARY_DIR)/$(BINARY_NAME)

run-grpc: build
	@echo "Running gRPC server..."
	./$(BINARY_DIR)/$(BINARY_NAME) -grpc 1

# Run tests
test:
	@echo "Running tests..."
	mockery
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	mockery
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	$(GOCMD) get github.com/vektra/mockery/v3@v3.6.3
	$(GOCMD) install github.com/vektra/mockery/v3@v3.6.3
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install buf.build/cmd/buf@latest
	@echo "Dependencies ready"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

# Generate protobuf code (requires buf)
proto:
	@echo "Generating protobuf code..."
	@which buf > /dev/null 2>&1 || (echo "buf not installed. Install with: go install buf.build/cmd/buf@latest" && exit 1)
	buf generate
	@echo "Protobuf code generated"

# Generate Swagger documentation (requires swag)
swagger:
	@echo "Generating Swagger documentation..."
	@which swag > /dev/null 2>&1 || (echo "swag not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest" && exit 1)
	swag init -g internal/transport/rest/rest.go -o internal/transport/rest/docs
	@echo "Swagger docs generated in internal/transport/rest/docs"

# Rename module for publishing
rename-module:
	@./scripts/rename-module.sh

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Format, test, and build (default)"
	@echo "  build          - Build the application and test client"
	@echo "  run            - Build and run the application"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter (requires golangci-lint)"
	@echo "  proto          - Generate protobuf code (requires buf)"
	@echo "  swagger        - Generate Swagger documentation (requires swag)"
	@echo "  rename-module  - Rename module for GitHub publishing"
	@echo "  help           - Show this help message"
