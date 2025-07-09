# Validator Service Makefile

.PHONY: help build build-server build-cli docker-build docker-run test clean deps

# Variables
VERSION ?= 1.0.0
DOCKER_TAG ?= validator:$(VERSION)
SERVER_BINARY = server/validator-server
CLI_BINARY = cmd/validator

# Default target
help:
	@echo "Available targets:"
	@echo "  build         - Build both server and CLI"
	@echo "  build-server  - Build server binary"
	@echo "  build-cli     - Build CLI binary"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download dependencies"

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build both binaries
build: build-server build-cli

# Build server binary
build-server:
	@echo "Building server binary..."
	cd server && go build -o validator-server -ldflags "-X main.Version=$(VERSION)" .

# Build CLI binary
build-cli:
	@echo "Building CLI binary..."
	cd cmd && go build -o validator -ldflags "-X main.Version=$(VERSION)" .

# Build Docker image
docker-build: build-server
	@echo "Building Docker image..."
	sudo docker build -t $(DOCKER_TAG) .

# Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	sudo docker run -p 8080:8080 --rm $(DOCKER_TAG)

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(SERVER_BINARY)
	rm -f $(CLI_BINARY)
	sudo docker rmi $(DOCKER_TAG) 2>/dev/null || true

# Run server locally (for development)
run-server: build-server
	@echo "Running server locally..."
	./$(SERVER_BINARY)

# Test CLI against running server
test-cli: build-cli
	@echo "Testing CLI..."
	./$(CLI_BINARY) --help