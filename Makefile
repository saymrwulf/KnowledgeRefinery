# Knowledge Refinery — Makefile
# ============================================================

.PHONY: all build install test clean daemon app-run daemon-run help

ROOT := $(shell pwd)
DAEMON_DIR := $(ROOT)/daemon-go
APP_DIR := $(ROOT)/apps/macos/KnowledgeRefinery

# ── Default: build everything ──
all: build

# ── Full build: Go daemon + SwiftUI app bundle ──
build:
	@bash scripts/build.sh

# ── Install to /Applications ──
install:
	@bash scripts/install.sh

# ── Build Go daemon only ──
daemon:
	@echo "Building Go daemon..."
	@cd $(DAEMON_DIR) && go build -o knowledge-refinery-daemon .
	@echo "  ✓ daemon-go/knowledge-refinery-daemon"

# ── Run app in development mode (swift run) ──
app-run: daemon
	@cd $(APP_DIR) && swift run

# ── Run daemon standalone ──
daemon-run: daemon
	@cd $(DAEMON_DIR) && ./knowledge-refinery-daemon

# ── Run all tests ──
test:
	@echo "Running Go daemon tests..."
	@cd $(DAEMON_DIR) && go test ./... -count=1 -short
	@echo ""
	@echo "Building SwiftUI app..."
	@cd $(APP_DIR) && swift build 2>&1 | tail -3
	@echo ""
	@echo "All checks passed."

# ── Clean build artifacts ──
clean:
	@echo "Cleaning..."
	@rm -rf $(ROOT)/dist
	@rm -rf $(APP_DIR)/.build
	@rm -f $(DAEMON_DIR)/knowledge-refinery-daemon
	@echo "  ✓ Clean"

# ── Help ──
help:
	@echo "Knowledge Refinery — Build Commands"
	@echo ""
	@echo "  make build        Build Go daemon + .app bundle (to dist/)"
	@echo "  make install      Full install to /Applications"
	@echo "  make test         Run Go tests + Swift build check"
	@echo "  make daemon       Build Go daemon binary only"
	@echo "  make app-run      Run app in dev mode (builds daemon first)"
	@echo "  make daemon-run   Run daemon standalone"
	@echo "  make clean        Remove build artifacts"
	@echo ""
