#!/usr/bin/env bash
# ============================================================
# Knowledge Refinery — Build Script
# Creates a distributable macOS .app bundle with embedded Go daemon
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="Knowledge Refinery"
BUNDLE_ID="com.knowledge-refinery.app"
VERSION="0.2.0"
BUILD_DIR="$ROOT/dist"
APP_DIR="$BUILD_DIR/${APP_NAME}.app"
CONTENTS="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"

echo "=== Knowledge Refinery Build ==="
echo "Root: $ROOT"
echo ""

# ── Step 1: Clean previous build ──
rm -rf "$BUILD_DIR"
mkdir -p "$MACOS_DIR" "$RESOURCES"

# ── Step 2: Build Go daemon (release) ──
echo "[1/5] Building Go daemon..."
cd "$ROOT/daemon-go"
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$MACOS_DIR/knowledge-refinery-daemon" .
DAEMON_SIZE=$(du -h "$MACOS_DIR/knowledge-refinery-daemon" | cut -f1)
echo "       Daemon binary: $DAEMON_SIZE"

# ── Step 3: Build SwiftUI app (release) ──
echo "[2/5] Building SwiftUI app (release)..."
cd "$ROOT/apps/macos/KnowledgeRefinery"
swift build -c release 2>&1 | tail -3

# ── Step 4: Assemble .app bundle ──
echo "[3/5] Assembling app bundle..."

# Copy Swift binary
cp .build/release/KnowledgeRefinery "$MACOS_DIR/KnowledgeRefinery-bin"

# Copy SPM resource bundle (contains WebGPU files + icon)
cp -R .build/release/KnowledgeRefinery_KnowledgeRefinery.bundle "$RESOURCES/"

# Copy shared prompts
cp -R "$ROOT/shared" "$RESOURCES/"

# Copy app icon to Resources root for Info.plist
cp "$ROOT/apps/macos/KnowledgeRefinery/Sources/Resources/AppIcon.icns" "$RESOURCES/AppIcon.icns"

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
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>15.0</string>
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
cat > "$MACOS_DIR/KnowledgeRefinery" << 'LAUNCHER'
#!/bin/bash
# Knowledge Refinery Launcher
DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES="$(cd "$DIR/../Resources" && pwd)"

# Tell the app where the Go daemon binary lives
export KR_DAEMON_DIR="$DIR"
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
