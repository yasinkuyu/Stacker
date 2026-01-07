#!/bin/bash

echo "Building Stacker..."

# Build for current platform (macOS arm64)
CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/stacker-mac-arm64 main.go
CGO_ENABLED=1 GOARCH=amd64 go build -ldflags="-s -w" -o dist/stacker-mac-amd64 main.go

# Linux (no tray for standalone)
# CGO_ENABLED=0 go build -ldflags="-s -w" -tags no_tray -o dist/stacker-linux-amd64 main.go

# Windows
# CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -tags no_tray -o dist/stacker-windows-amd64.exe main.go

# Create macOS app bundles
echo ""
# Create macOS app bundles
echo ""
echo "📦 Creating macOS Universal .app bundle..."

# Generate AppIcon.icns
echo "🎨 Generating AppIcon.icns..."
rm -f dist/AppIcon.icns
ICONSET="dist/Stacker.iconset"
mkdir -p "$ICONSET"
SOURCE_ICON="internal/web/app_icon_1024.png"

# Check if source icon exists
if [ -f "$SOURCE_ICON" ]; then
    sips -z 16 16     -s format png "$SOURCE_ICON" --out "$ICONSET/icon_16x16.png" > /dev/null
    sips -z 32 32     -s format png "$SOURCE_ICON" --out "$ICONSET/icon_16x16@2x.png" > /dev/null
    sips -z 32 32     -s format png "$SOURCE_ICON" --out "$ICONSET/icon_32x32.png" > /dev/null
    sips -z 64 64     -s format png "$SOURCE_ICON" --out "$ICONSET/icon_32x32@2x.png" > /dev/null
    sips -z 128 128   -s format png "$SOURCE_ICON" --out "$ICONSET/icon_128x128.png" > /dev/null
    sips -z 256 256   -s format png "$SOURCE_ICON" --out "$ICONSET/icon_128x128@2x.png" > /dev/null
    sips -z 256 256   -s format png "$SOURCE_ICON" --out "$ICONSET/icon_256x256.png" > /dev/null
    sips -z 512 512   -s format png "$SOURCE_ICON" --out "$ICONSET/icon_256x256@2x.png" > /dev/null
    sips -z 512 512   -s format png "$SOURCE_ICON" --out "$ICONSET/icon_512x512.png" > /dev/null
    sips -z 1024 1024 -s format png "$SOURCE_ICON" --out "$ICONSET/icon_512x512@2x.png" > /dev/null

    iconutil -c icns "$ICONSET" -o dist/AppIcon.icns
    rm -rf "$ICONSET"
else
    echo "⚠️  High-res icon not found ($SOURCE_ICON). Using fallback."
fi

# Create Universal Binary using Lipo
echo "🔗 Creating Universal Binary (arm64 + amd64)..."
lipo -create -output dist/stacker-universal dist/stacker-mac-arm64 dist/stacker-mac-amd64

# Universal .app
mkdir -p dist/Stacker.app/Contents/{MacOS,Resources}
cp dist/stacker-universal dist/Stacker.app/Contents/MacOS/stacker
chmod +x dist/Stacker.app/Contents/MacOS/stacker

# Copy AppIcon.icns if generated, otherwise fallback
if [ -f "dist/AppIcon.icns" ]; then
    cp dist/AppIcon.icns dist/Stacker.app/Contents/Resources/AppIcon.icns
else
    cp internal/web/logo.png dist/Stacker.app/Contents/Resources/AppIcon.png
fi

cat > dist/Stacker.app/Contents/Info.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDisplayName</key>
    <string>Stacker</string>
    <key>CFBundleExecutable</key>
    <string>stacker</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>com.insya.stacker</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Stacker</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleVersion</key>
    <string>5</string>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.developer-tools</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <true/>
</dict>
</plist>
EOF

echo ""
echo "✅ Build complete! Binaries available in dist/"
echo ""
echo "macOS (GUI App):"
echo "  - dist/Stacker-arm64.app (Apple Silicon) - Double-click to run"
echo "  - dist/Stacker-amd64.app (Intel) - Double-click to run"
echo ""
echo "macOS (Terminal):"
echo "  - dist/stacker-mac-arm64 (Apple Silicon)"
echo "  - dist/stacker-mac-amd64 (Intel)"
echo ""
echo "Linux & Windows (Standalone, no tray):"
ls -lh dist/stacker-linux-amd64 dist/stacker-windows-amd64.exe 2>/dev/null
