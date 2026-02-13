#!/usr/bin/env bash
# ============================================================
# Knowledge Refinery — Fresh Machine Installer
# For macOS 15+ (Sequoia/Tahoe) on Apple Silicon
# ============================================================
set -euo pipefail

echo "========================================"
echo "  Knowledge Refinery Installer"
echo "  macOS · Apple Silicon"
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
echo "[1/5] Checking prerequisites..."

# macOS version
SW_VER=$(sw_vers -productVersion)
MAJOR=$(echo "$SW_VER" | cut -d. -f1)
if [ "$MAJOR" -ge 15 ]; then
    ok "macOS $SW_VER"
else
    fail "macOS 15+ required — found $SW_VER"
fi

# Architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    ok "Apple Silicon ($ARCH)"
else
    warn "Architecture: $ARCH (Apple Silicon recommended)"
fi

# Xcode Command Line Tools
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

# Go
if command -v go &>/dev/null; then
    GO_VER=$(go version 2>&1)
    ok "Go: $GO_VER"
else
    fail "Go not found. Install from https://go.dev/dl/ or: brew install go"
fi

echo ""

# ── Step 2: Run Go tests ──
echo "[2/5] Running Go daemon tests..."
cd "$ROOT/daemon-go"
go test ./... -count=1 -short 2>&1 | tail -15
ok "All Go tests passed"

echo ""

# ── Step 3: Build .app bundle ──
echo "[3/5] Building application..."
bash "$ROOT/scripts/build.sh"

echo ""

# ── Step 4: Install to /Applications ──
echo "[4/5] Installing to /Applications..."
APP_SRC="$ROOT/dist/Knowledge Refinery.app"
APP_DST="/Applications/Knowledge Refinery.app"

if [ -d "$APP_DST" ]; then
    warn "Existing installation found — replacing..."
    rm -rf "$APP_DST"
fi

cp -R "$APP_SRC" "$APP_DST"
ok "Installed to $APP_DST"

echo ""

# ── Step 5: Create data directory ──
echo "[5/5] Initializing data directory..."
KR_DIR="$HOME/.knowledge-refinery"
mkdir -p "$KR_DIR"
mkdir -p "$KR_DIR/workspaces"

if [ ! -f "$KR_DIR/workspaces.json" ]; then
    echo '[]' > "$KR_DIR/workspaces.json"
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
echo "       - Chat:       qwen3-4b-2507 (or similar small model)"
echo "       - Embeddings: nomic-embed-text-v1.5"
echo "    3. Start the LM Studio local server (port 1234)"
echo ""
echo "  Then:"
echo "    open '/Applications/Knowledge Refinery.app'"
echo ""
echo "  Or from Launchpad / Spotlight: 'Knowledge Refinery'"
echo ""
