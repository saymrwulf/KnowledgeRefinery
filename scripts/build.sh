#!/usr/bin/env bash
# ============================================================
# Knowledge Refinery — Build Script
# Creates a distributable macOS .app bundle
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="Knowledge Refinery"
BUNDLE_ID="com.knowledge-refinery.app"
VERSION="0.1.0"
BUILD_DIR="$ROOT/dist"
APP_DIR="$BUILD_DIR/${APP_NAME}.app"
CONTENTS="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
DAEMON_BUNDLE="$RESOURCES/daemon"

echo "=== Knowledge Refinery Build ==="
echo "Root: $ROOT"
echo ""

# ── Step 1: Clean previous build ──
rm -rf "$BUILD_DIR"
mkdir -p "$MACOS_DIR" "$RESOURCES"

# ── Step 2: Build SwiftUI app (release) ──
echo "[1/5] Building SwiftUI app (release)..."
cd "$ROOT/apps/macos/KnowledgeRefinery"
swift build -c release 2>&1 | tail -3
cp .build/release/KnowledgeRefinery "$MACOS_DIR/KnowledgeRefinery"

# ── Step 3: Copy WebGPU resources ──
echo "[2/5] Bundling WebGPU resources..."
mkdir -p "$RESOURCES/WebGPU"
cp .build/release/KnowledgeRefinery_KnowledgeRefinery.bundle/universe.html "$RESOURCES/WebGPU/"
cp .build/release/KnowledgeRefinery_KnowledgeRefinery.bundle/universe.js  "$RESOURCES/WebGPU/"
cp .build/release/KnowledgeRefinery_KnowledgeRefinery.bundle/universe.wgsl "$RESOURCES/WebGPU/"

# ── Step 4: Bundle Python daemon ──
echo "[3/5] Bundling Python daemon..."
mkdir -p "$DAEMON_BUNDLE"
# Copy daemon source
cp -R "$ROOT/daemon/knowledge_refinery" "$DAEMON_BUNDLE/"
cp "$ROOT/daemon/pyproject.toml" "$DAEMON_BUNDLE/"
# Copy shared prompts/models/schemas
cp -R "$ROOT/shared" "$DAEMON_BUNDLE/"

# ── Step 5: Create Info.plist ──
echo "[4/5] Creating Info.plist..."
cat > "$CONTENTS/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>KnowledgeRefinery</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>26.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.productivity</string>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsLocalNetworking</key>
        <true/>
    </dict>
</dict>
</plist>
PLIST

# ── Step 6: Create launcher wrapper ──
echo "[5/5] Creating launcher..."
# The launcher script ensures the daemon Python environment is available
# and sets the right paths before launching the main binary
mv "$MACOS_DIR/KnowledgeRefinery" "$MACOS_DIR/KnowledgeRefinery-bin"
cat > "$MACOS_DIR/KnowledgeRefinery" << 'LAUNCHER'
#!/bin/bash
# Knowledge Refinery Launcher
# Sets up environment and launches the app binary
DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES="$(cd "$DIR/../Resources" && pwd)"

# Export daemon location so the app can find it
export KR_DAEMON_DIR="$RESOURCES/daemon"
export KR_RESOURCES_DIR="$RESOURCES"

exec "$DIR/KnowledgeRefinery-bin" "$@"
LAUNCHER
chmod +x "$MACOS_DIR/KnowledgeRefinery"

# ── Summary ──
APP_SIZE=$(du -sh "$APP_DIR" | cut -f1)
echo ""
echo "=== Build Complete ==="
echo "App:  $APP_DIR"
echo "Size: $APP_SIZE"
echo ""
echo "To install: drag '${APP_NAME}.app' to /Applications"
echo "Or run:     open \"$APP_DIR\""
