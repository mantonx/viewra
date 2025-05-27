# Viewra Plugin System Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build parameters
BINARY_NAME=viewra
PLUGIN_DIR=data/plugins

# Protobuf parameters
PROTOC=protoc
PROTO_DIR=internal/plugins/proto
PROTO_FILES=$(PROTO_DIR)/*.proto

.PHONY: all build clean test test-verbose test-coverage proto build-plugins install-protoc deps

# Default target
all: deps proto build-plugins build test

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

# Build main application
build:
	@echo "Building main application..."
	@$(GOBUILD) -o $(BINARY_NAME) ./cmd/viewra

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# Build all plugins
build-plugins:
	@echo "Building plugins..."
	@for plugin_dir in $(PLUGIN_DIR)/*/; do \
		if [ -f "$$plugin_dir/plugin.cue" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			echo "Building plugin: $$plugin_name"; \
			cd "$$plugin_dir" && \
			$(GOMOD) download && \
			$(GOMOD) tidy && \
			$(GOBUILD) -o "$$plugin_name" . && \
			cd - > /dev/null; \
		fi \
	done

# Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./internal/plugins/...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@$(GOTEST) -v -race ./internal/plugins/...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./internal/plugins/...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./internal/plugins/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@for plugin_dir in $(PLUGIN_DIR)/*/; do \
		if [ -f "$$plugin_dir/plugin.cue" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			rm -f "$$plugin_dir/$$plugin_name"; \
		fi \
	done
	@find $(PROTO_DIR) -name "*.pb.go" -delete

# Install protobuf tools
install-protoc:
	@echo "Installing protobuf tools..."
	@$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Check code formatting
fmt:
	@echo "Checking code formatting..."
	@$(GOCMD) fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Validate plugin configurations
validate-plugins:
	@echo "Validating plugin configurations..."
	@for plugin_dir in $(PLUGIN_DIR)/*/; do \
		if [ -f "$$plugin_dir/plugin.cue" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			echo "Validating plugin: $$plugin_name"; \
			cue vet "$$plugin_dir/plugin.cue"; \
		fi \
	done

# Development target - quick rebuild for testing
dev: clean proto build-plugins build

# Plugin development helper - rebuild specific plugin
build-plugin:
	@if [ -z "$(PLUGIN)" ]; then \
		echo "Usage: make build-plugin PLUGIN=plugin_name"; \
		exit 1; \
	fi
	@plugin_dir="$(PLUGIN_DIR)/$(PLUGIN)"; \
	if [ -f "$$plugin_dir/plugin.cue" ]; then \
		echo "Building plugin: $(PLUGIN)"; \
		cd "$$plugin_dir" && \
		$(GOMOD) download && \
		$(GOMOD) tidy && \
		$(GOBUILD) -o "$(PLUGIN)" . && \
		cd - > /dev/null; \
	else \
		echo "Plugin not found: $(PLUGIN)"; \
		exit 1; \
	fi

# Integration test target
test-integration:
	@echo "Running integration tests..."
	@$(GOTEST) -v -tags=integration ./internal/plugins/...

# Help target
help:
	@echo "Available targets:"
	@echo "  all              - Build everything (default)"
	@echo "  build            - Build main application"
	@echo "  proto            - Generate protobuf files"
	@echo "  build-plugins    - Build all plugins"
	@echo "  build-plugin     - Build specific plugin (use PLUGIN=name)"
	@echo "  test             - Run tests"
	@echo "  test-verbose     - Run tests with verbose output"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  benchmark        - Run benchmarks"
	@echo "  test-integration - Run integration tests"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Install dependencies"
	@echo "  install-protoc   - Install protobuf tools"
	@echo "  fmt              - Check code formatting"
	@echo "  lint             - Run linter"
	@echo "  validate-plugins - Validate plugin configurations"
	@echo "  dev              - Quick development rebuild"
	@echo "  help             - Show this help" 