#!/bin/bash

echo "Building stacker-app..."

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/stacker-app-darwin-arm64 main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/stacker-app-darwin-amd64 main.go

# Linux
GOOS=linux GOARCH=amd64 go build -o dist/stacker-app-linux-amd64 main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/stacker-app-windows-amd64.exe main.go

echo "Build complete! Binaries available in dist/"
ls -lh dist/
