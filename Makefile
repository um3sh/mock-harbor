# Binary name
BINARY_NAME=mock-harbor

# Main build directory
BUILD_DIR=dist

# Main package path
MAIN_PKG=./cmd/server

# Version from git tag (if available)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Architectures and platforms to build for
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

# Default make target
.PHONY: all
all: clean build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

# Build for all platforms
.PHONY: build
build:
	@echo "Building ${BINARY_NAME} version ${VERSION}..."
	@mkdir -p $(BUILD_DIR)
	@$(MAKE) darwin-amd64 darwin-arm64 linux-amd64 linux-arm64

# Individual build targets
.PHONY: darwin-amd64
darwin-amd64:
	@echo "Building for macOS (x86_64)..."
	@GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-X 'main.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PKG)

.PHONY: darwin-arm64
darwin-arm64:
	@echo "Building for macOS (arm64)..."
	@GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-X 'main.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PKG)

.PHONY: linux-amd64
linux-amd64:
	@echo "Building for Linux (x86_64)..."
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-X 'main.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PKG)

.PHONY: linux-arm64
linux-arm64:
	@echo "Building for Linux (arm64)..."
	@GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-X 'main.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PKG)

# Create SHA256 checksums for all binaries
.PHONY: checksums
checksums:
	@echo "Generating checksums..."
	@cd $(BUILD_DIR) && sha256sum $(BINARY_NAME)-* > SHA256SUMS

# Create a release (build and generate checksums)
.PHONY: release
release: build checksums
	@echo "Release artifacts created in $(BUILD_DIR)"
