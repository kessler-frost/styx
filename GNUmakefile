PLUGIN_NAME := apple-container
PLUGIN_DIR := $(shell pwd)/plugins
CLI_NAME := styx
BIN_DIR := $(shell pwd)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build build-cli build-all clean test dev install

# Build the driver plugin
build:
	@echo "Building driver..."
	go build -o $(PLUGIN_DIR)/$(PLUGIN_NAME) ./driver

# Build the CLI
build-cli:
	@echo "Building CLI..."
	go build -o $(BIN_DIR)/$(CLI_NAME) ./cmd/styx

# Build everything
build-all: build build-cli

clean:
	rm -rf $(PLUGIN_DIR)
	rm -rf $(BIN_DIR)
	rm -rf dist/

test:
	go test -v ./...

# Run Nomad in dev mode with the plugin
dev: build
	@echo "Starting Nomad in dev mode..."
	@echo "Plugin directory: $(PLUGIN_DIR)"
	nomad agent -dev -plugin-dir=$(PLUGIN_DIR) -config=example/agent.hcl

# Install to user directories (no sudo required)
install: build-all
	@echo "Installing styx..."
	mkdir -p $(HOME)/.local/bin
	mkdir -p $(HOME)/.local/lib/styx/plugins
	cp $(PLUGIN_DIR)/$(PLUGIN_NAME) $(HOME)/.local/lib/styx/plugins/
	cp $(BIN_DIR)/$(CLI_NAME) $(HOME)/.local/bin/
	@echo "Installed to ~/.local/bin and ~/.local/lib/styx/plugins"
	@echo "Ensure ~/.local/bin is in your PATH"

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod tidy

# Build for release
release:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(PLUGIN_NAME)_darwin_arm64 ./driver
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(PLUGIN_NAME)_darwin_amd64 ./driver
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(CLI_NAME)_darwin_arm64 ./cmd/styx
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(CLI_NAME)_darwin_amd64 ./cmd/styx
