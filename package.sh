#!/bin/bash

# Ensure dist directory exists
if [ ! -d "dist" ]; then
    echo "❌ dist/ directory not found. Please run build.sh first."
    exit 1
fi

echo "📦 Packaging Stacker for macOS..."

APP_CERT="Apple Distribution: Yasin Kuyu (6H6NRZC3V6)"
INSTALLER_CERT="3rd Party Mac Developer Installer: Yasin Kuyu (6H6NRZC3V6)"
PROVISIONING_PROFILE="Stacker.provisionprofile"

# Sign and Package Universal App
if [ -d "dist/Stacker.app" ]; then
    echo "  • Processing dist/Stacker.app (Universal)..."
    
    # Embed Provisioning Profile if present
    if [ -f "$PROVISIONING_PROFILE" ]; then
        echo "  • Embedding Provisioning Profile..."
        cp "$PROVISIONING_PROFILE" "dist/Stacker.app/Contents/embedded.provisionprofile"
    else
        echo "⚠️  Provisioning Profile ($PROVISIONING_PROFILE) not found! App Store upload may fail."
    fi

    echo "  • Signing dist/Stacker.app..."
    codesign --force --verify --verbose --sign "$APP_CERT" --options runtime --timestamp --entitlements "entitlements.plist" "dist/Stacker.app"
    
    echo "  • Creating dist/Stacker.pkg..."
    productbuild --sign "$INSTALLER_CERT" \
                 --component "dist/Stacker.app" \
                 "/Applications" \
                 "dist/Stacker.pkg"
else
    echo "❌ dist/Stacker.app not found. Run build.sh first."
fi

echo ""
echo "✅ Packaging complete!"
ls -lh dist/*.pkg 2>/dev/null
