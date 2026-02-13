#!/usr/bin/env bash
# ============================================================
# Knowledge Refinery — Fresh Machine Installer
# For macOS Tahoe (26.x) on Apple Silicon
# ============================================================
set -euo pipefail

echo "========================================"
echo "  Knowledge Refinery Installer"
echo "  macOS Tahoe · Apple Silicon"
echo "========================================"
echo ""

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# ── Colors ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

ok()   { echo -e "  ${GREEN}✓${NC} $1"; }
warn() { echo -e "  ${YELLOW}!${NC} $1"; }
fail() { echo -e "  ${RED}✗${NC} $1"; exit 1; }
info() { echo -e "  ${CYAN}→${NC} $1"; }

# ── Step 1: Check prerequisites ──
echo "[1/6] Checking prerequisites..."

# macOS version
SW_VER=$(sw_vers -productVersion)
MAJOR=$(echo "$SW_VER" | cut -d. -f1)
if [ "$MAJOR" -ge 26 ]; then
    ok "macOS Tahoe ($SW_VER)"
else
    fail "macOS Tahoe (26.x) required — found $SW_VER"
fi

# Architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    ok "Apple Silicon ($ARCH)"
else
    warn "Architecture: $ARCH (Apple Silicon recommended)"
fi

# Xcode Command Line Tools / Xcode
if xcode-select -p &>/dev/null; then
    ok "Xcode/Command Line Tools installed"
else
    info "Installing Xcode Command Line Tools..."
    xcode-select --install
    echo "    Please complete the installation dialog, then re-run this script."
    exit 1
fi

# Swift
if command -v swift &>/dev/null; then
    SWIFT_VER=$(swift --version 2>&1 | head -1)
    ok "Swift: $SWIFT_VER"
else
    fail "Swift not found — install Xcode or Xcode Command Line Tools"
fi

# Python 3.12+
PYTHON=""
for cmd in python3.14 python3.13 python3.12 python3; do
    if command -v "$cmd" &>/dev/null; then
        PY_VER=$("$cmd" --version 2>&1 | grep -oE '[0-9]+\.[0-9]+')
        PY_MAJOR=$(echo "$PY_VER" | cut -d. -f1)
        PY_MINOR=$(echo "$PY_VER" | cut -d. -f2)
        if [ "$PY_MAJOR" -ge 3 ] && [ "$PY_MINOR" -ge 12 ]; then
            PYTHON="$cmd"
            break
        fi
    fi
done

if [ -n "$PYTHON" ]; then
    ok "Python: $($PYTHON --version)"
else
    fail "Python 3.12+ required. Install from python.org or: brew install python@3.14"
fi

echo ""

# ── Step 2: Set up Python virtual environment ──
echo "[2/6] Setting up Python virtual environment..."
DAEMON_DIR="$ROOT/daemon"
VENV_DIR="$DAEMON_DIR/.venv"

if [ -d "$VENV_DIR" ]; then
    ok "Virtual environment exists at $VENV_DIR"
else
    info "Creating virtual environment..."
    "$PYTHON" -m venv "$VENV_DIR"
    ok "Virtual environment created"
fi

info "Installing Python dependencies..."
"$VENV_DIR/bin/pip" install --upgrade pip -q
"$VENV_DIR/bin/pip" install -e "$DAEMON_DIR" -q
"$VENV_DIR/bin/pip" install -e "$DAEMON_DIR[dev]" -q
ok "Python dependencies installed"

echo ""

# ── Step 3: Build the SwiftUI app ──
echo "[3/6] Building SwiftUI application..."
cd "$ROOT/apps/macos/KnowledgeRefinery"
swift build -c release 2>&1 | tail -3
ok "SwiftUI app built (release)"

echo ""

# ── Step 4: Create .app bundle ──
echo "[4/6] Creating application bundle..."
bash "$ROOT/scripts/build.sh"

echo ""

# ── Step 5: Install to /Applications ──
echo "[5/6] Installing to /Applications..."
APP_SRC="$ROOT/dist/Knowledge Refinery.app"
APP_DST="/Applications/Knowledge Refinery.app"

if [ -d "$APP_DST" ]; then
    warn "Existing installation found — replacing..."
    rm -rf "$APP_DST"
fi

cp -R "$APP_SRC" "$APP_DST"
ok "Installed to $APP_DST"

echo ""

# ── Step 6: Create data directory ──
echo "[6/6] Initializing data directory..."
KR_DIR="$HOME/.knowledge-refinery"
mkdir -p "$KR_DIR"
mkdir -p "$KR_DIR/workspaces"

if [ ! -f "$KR_DIR/workspaces.json" ]; then
    echo '{"workspaces":[]}' > "$KR_DIR/workspaces.json"
    ok "Created workspaces.json"
else
    ok "workspaces.json already exists"
fi

ok "Data directory ready: $KR_DIR"

echo ""
echo "========================================"
echo "  Installation Complete!"
echo "========================================"
echo ""
echo "  Before launching:"
echo "    1. Install LM Studio from https://lmstudio.ai"
echo "    2. Load models in LM Studio:"
echo "       - Chat:       gemma-3-4b (or similar)"
echo "       - Embeddings: nomic-embed-text-v1.5"
echo "    3. Start the LM Studio local server (port 1234)"
echo ""
echo "  Then:"
echo "    open '/Applications/Knowledge Refinery.app'"
echo ""
echo "  Or from Launchpad / Spotlight: 'Knowledge Refinery'"
echo ""
