#!/bin/bash

echo "Building Stacker..."

# Build for current platform (macOS arm64)
CGO_ENABLED=1 go build -o dist/stacker-mac-arm64 main.go
CGO_ENABLED=1 GOARCH=amd64 go build -o dist/stacker-mac-amd64 main.go

# Linux (no tray for standalone)
CGO_ENABLED=0 go build -tags no_tray -o dist/stacker-linux-amd64 main.go

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags no_tray -o dist/stacker-windows-amd64.exe main.go

# Create macOS app bundles
echo ""
echo "📦 Creating macOS .app bundles..."

# ARM64 .app
mkdir -p dist/Stacker-arm64.app/Contents/{MacOS,Resources}
cp dist/stacker-mac-arm64 dist/Stacker-arm64.app/Contents/MacOS/stacker
chmod +x dist/Stacker-arm64.app/Contents/MacOS/stacker
cp internal/web/logo.png dist/Stacker-arm64.app/Contents/Resources/AppIcon.png
cat > dist/Stacker-arm64.app/Contents/Info.plist << 'EOF'
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
    <string>com.yasinkuyu.stacker</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Stacker</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# AMD64 .app
mkdir -p dist/Stacker-amd64.app/Contents/{MacOS,Resources}
cp dist/stacker-mac-amd64 dist/Stacker-amd64.app/Contents/MacOS/stacker
chmod +x dist/Stacker-amd64.app/Contents/MacOS/stacker
cp internal/web/logo.png dist/Stacker-amd64.app/Contents/Resources/AppIcon.png
cp dist/Stacker-arm64.app/Contents/Info.plist dist/Stacker-amd64.app/Contents/Info.plist

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
