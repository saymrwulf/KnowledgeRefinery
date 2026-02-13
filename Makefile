# Knowledge Refinery — Makefile
# ============================================================

.PHONY: all build install test clean daemon-setup app-build app-run

ROOT := $(shell pwd)
DAEMON_DIR := $(ROOT)/daemon
APP_DIR := $(ROOT)/apps/macos/KnowledgeRefinery
VENV := $(DAEMON_DIR)/.venv
PYTHON := $(VENV)/bin/python

# ── Default: build everything ──
all: build

# ── Full build: daemon setup + app bundle ──
build: daemon-setup app-build
	@echo ""
	@echo "Build complete. Run 'make install' to copy to /Applications."

# ── Install to /Applications ──
install:
	@bash scripts/install.sh

# ── Set up Python daemon venv + deps ──
daemon-setup:
	@echo "Setting up Python daemon..."
	@test -d $(VENV) || python3 -m venv $(VENV)
	@$(VENV)/bin/pip install --upgrade pip -q
	@$(VENV)/bin/pip install -e $(DAEMON_DIR) -q
	@$(VENV)/bin/pip install -e "$(DAEMON_DIR)[dev]" -q
	@echo "  ✓ Daemon dependencies installed"

# ── Build SwiftUI .app bundle ──
app-build:
	@bash scripts/build.sh

# ── Run app in development mode (swift run) ──
app-run:
	@cd $(APP_DIR) && swift run

# ── Run daemon in development mode ──
daemon-run:
	@cd $(DAEMON_DIR) && $(PYTHON) -m knowledge_refinery.main

# ── Run tests ──
test:
	@echo "Running daemon tests..."
	@cd $(DAEMON_DIR) && $(VENV)/bin/python -m pytest tests/ -x -q
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
	@echo "  ✓ Clean"

# ── Help ──
help:
	@echo "Knowledge Refinery — Build Commands"
	@echo ""
	@echo "  make build        Build daemon + app bundle (to dist/)"
	@echo "  make install      Full install to /Applications"
	@echo "  make test         Run daemon tests + Swift build check"
	@echo "  make app-run      Run app in dev mode (swift run)"
	@echo "  make daemon-run   Run daemon in dev mode"
	@echo "  make clean        Remove build artifacts"
	@echo ""
