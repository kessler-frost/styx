PLUGIN_NAME := apple-container
PLUGIN_DIR := $(shell pwd)/plugins
CLI_NAME := styx
BIN_DIR := $(shell pwd)/bin

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

# Install styx and plugin to system locations (requires sudo)
install: build-all
	@echo "Installing styx..."
	sudo mkdir -p /usr/local/lib/styx/plugins
	sudo cp $(PLUGIN_DIR)/$(PLUGIN_NAME) /usr/local/lib/styx/plugins/
	sudo chmod 755 /usr/local/lib/styx/plugins/$(PLUGIN_NAME)
	sudo cp $(BIN_DIR)/$(CLI_NAME) /usr/local/bin/
	sudo chmod 755 /usr/local/bin/$(CLI_NAME)
	@echo "Installed successfully!"
	@echo "  - Plugin: /usr/local/lib/styx/plugins/$(PLUGIN_NAME)"
	@echo "  - CLI: /usr/local/bin/$(CLI_NAME)"

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
	GOOS=darwin GOARCH=arm64 go build -o dist/$(PLUGIN_NAME)_darwin_arm64 ./driver
	GOOS=darwin GOARCH=amd64 go build -o dist/$(PLUGIN_NAME)_darwin_amd64 ./driver
	GOOS=darwin GOARCH=arm64 go build -o dist/$(CLI_NAME)_darwin_arm64 ./cmd/styx
	GOOS=darwin GOARCH=amd64 go build -o dist/$(CLI_NAME)_darwin_amd64 ./cmd/styx
