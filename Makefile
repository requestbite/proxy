# RequestBite Proxy Makefile
# Cross-platform build automation for macOS, Linux, and Windows

# Extract version from main.go
VERSION := $(shell grep 'Version.*=' main.go | head -1 | sed 's/.*"\(.*\)".*/\1/')

# Binary name
BINARY_NAME := requestbite-proxy

# Build metadata
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.BuildTime=$(BUILD_TIME)' \
	-X 'main.GitCommit=$(GIT_COMMIT)'

BUILD_FLAGS := -ldflags="$(LDFLAGS)" -trimpath

# Target platforms
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	windows/amd64

# Output directory
DIST_DIR := dist

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_BLUE := \033[34m

.PHONY: all build build-all release clean install help

# Default target
all: build

# Build for current platform (development)
build:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Building $(BINARY_NAME) v$(VERSION) for current platform...$(COLOR_RESET)"
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(BINARY_NAME) .
	@echo "$(COLOR_GREEN)✓ Build complete: $(BINARY_NAME)$(COLOR_RESET)"

# Build for all platforms
build-all: clean
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Building $(BINARY_NAME) v$(VERSION) for all platforms...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@$(foreach platform,$(PLATFORMS),\
		$(call build_platform,$(platform)))
	@echo "$(COLOR_GREEN)✓ All builds complete$(COLOR_RESET)"
	@echo ""
	@echo "Built binaries:"
	@ls -lh $(DIST_DIR)/$(BINARY_NAME)-*

# Build for a specific platform (internal function)
define build_platform
	$(eval OS := $(word 1,$(subst /, ,$(1))))
	$(eval ARCH := $(word 2,$(subst /, ,$(1))))
	$(eval OUTPUT := $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$(OS)-$(ARCH)$(if $(filter windows,$(OS)),.exe,))
	@echo "  Building for $(OS)/$(ARCH)..."
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build $(BUILD_FLAGS) -o $(OUTPUT) .
endef

# Create release archives and checksums
release: build-all
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Creating release archives...$(COLOR_RESET)"
	@cd $(DIST_DIR) && \
	for binary in $(BINARY_NAME)-$(VERSION)-*; do \
		if [ -f "$$binary" ]; then \
			base=$$(basename "$$binary"); \
			os_arch=$$(echo "$$base" | sed 's/$(BINARY_NAME)-$(VERSION)-//'); \
			\
			if echo "$$os_arch" | grep -q "windows"; then \
				archive="$(BINARY_NAME)-$(VERSION)-$${os_arch%.exe}.zip"; \
				echo "  Creating $$archive..."; \
				cp ../LICENSE . 2>/dev/null || true; \
				cp ../README.md . 2>/dev/null || true; \
				zip -q "$$archive" "$$binary" LICENSE README.md 2>/dev/null || zip -q "$$archive" "$$binary"; \
				rm -f LICENSE README.md; \
			else \
				archive="$(BINARY_NAME)-$(VERSION)-$$os_arch.tar.gz"; \
				echo "  Creating $$archive..."; \
				temp_dir="$(BINARY_NAME)"; \
				mkdir -p "$$temp_dir"; \
				cp "$$binary" "$$temp_dir/$(BINARY_NAME)"; \
				cp ../LICENSE "$$temp_dir/" 2>/dev/null || true; \
				cp ../README.md "$$temp_dir/" 2>/dev/null || true; \
				tar -czf "$$archive" "$$temp_dir"; \
				rm -rf "$$temp_dir"; \
			fi; \
		fi; \
	done
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Generating checksums...$(COLOR_RESET)"
	@cd $(DIST_DIR) && \
	if command -v shasum >/dev/null 2>&1; then \
		shasum -a 256 *.tar.gz *.zip 2>/dev/null > SHA256SUMS; \
	else \
		sha256sum *.tar.gz *.zip 2>/dev/null > SHA256SUMS; \
	fi
	@echo "$(COLOR_GREEN)✓ Release archives created$(COLOR_RESET)"
	@echo ""
	@echo "Release artifacts:"
	@ls -lh $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip 2>/dev/null || true
	@echo ""
	@echo "Checksums (SHA256SUMS):"
	@cat $(DIST_DIR)/SHA256SUMS

# Clean build artifacts
clean:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(DIST_DIR)
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

# Install locally for testing (to ~/.local/bin)
install: build
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Installing $(BINARY_NAME) to ~/.local/bin...$(COLOR_RESET)"
	@mkdir -p ~/.local/bin
	@cp $(BINARY_NAME) ~/.local/bin/
	@chmod +x ~/.local/bin/$(BINARY_NAME)
	@echo "$(COLOR_GREEN)✓ Installed to ~/.local/bin/$(BINARY_NAME)$(COLOR_RESET)"
	@echo ""
	@if echo "$$PATH" | grep -q "$$HOME/.local/bin"; then \
		echo "Ready to use: $(BINARY_NAME) --version"; \
	else \
		echo "⚠ Warning: ~/.local/bin is not in your PATH"; \
		echo "Add to PATH: export PATH=\"\$$HOME/.local/bin:\$$PATH\""; \
	fi

# Show version
version:
	@echo "$(BINARY_NAME) v$(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Git commit: $(GIT_COMMIT)"

# Show help
help:
	@echo "$(COLOR_BOLD)RequestBite Proxy - Build System$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Usage:$(COLOR_RESET)"
	@echo "  make [target]"
	@echo ""
	@echo "$(COLOR_BOLD)Targets:$(COLOR_RESET)"
	@echo "  build      - Build for current platform (default)"
	@echo "  build-all  - Build for all platforms (darwin/amd64, darwin/arm64, linux/amd64, windows/amd64)"
	@echo "  release    - Build all platforms and create release archives with checksums"
	@echo "  clean      - Remove all build artifacts"
	@echo "  install    - Install locally to ~/.local/bin (for testing)"
	@echo "  version    - Show version information"
	@echo "  help       - Show this help message"
	@echo ""
	@echo "$(COLOR_BOLD)Examples:$(COLOR_RESET)"
	@echo "  make build          # Quick build for development"
	@echo "  make build-all      # Build for all platforms"
	@echo "  make release        # Create release archives"
	@echo "  make install        # Install locally"
	@echo ""
	@echo "$(COLOR_BOLD)Current version:$(COLOR_RESET) $(VERSION)"
