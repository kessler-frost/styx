PLUGIN_NAME := apple-container
PLUGIN_DIR := ./plugins

.PHONY: build clean test dev

build:
	@echo "Building driver..."
	go build -o $(PLUGIN_DIR)/$(PLUGIN_NAME) ./driver

clean:
	rm -rf $(PLUGIN_DIR)
	rm -rf dist/

test:
	go test -v ./...

# Run Nomad in dev mode with the plugin
dev: build
	@echo "Starting Nomad in dev mode..."
	@echo "Plugin directory: $(PLUGIN_DIR)"
	nomad agent -dev -plugin-dir=$(PLUGIN_DIR) -config=example/agent.hcl

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
