#!/bin/bash

echo "Building Stacker..."

echo ""
echo "📦 Building with System Tray (CGO enabled)..."
GOOS=darwin GOARCH=arm64 go build -o dist/stacker-darwin-arm64 main.go
GOOS=darwin GOARCH=amd64 go build -o dist/stacker-darwin-amd64 main.go
GOOS=linux GOARCH=amd64 go build -o dist/stacker-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o dist/stacker-windows-amd64.exe main.go

echo ""
echo "🚀 Building without System Tray (CGO disabled, single binary)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -tags no_tray -o dist/stacker-darwin-arm64-nocgo main.go
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags no_tray -o dist/stacker-darwin-amd64-nocgo main.go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags no_tray -o dist/stacker-linux-amd64-nocgo main.go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags no_tray -o dist/stacker-windows-amd64-nocgo.exe main.go

echo ""
echo "✅ Build complete! Binaries available in dist/"
echo ""
echo "With System Tray (requires OS dependencies):"
ls -lh dist/ | grep -v nocgo
echo ""
echo "Without System Tray (standalone, no dependencies):"
ls -lh dist/ | grep nocgo
