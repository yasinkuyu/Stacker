---
description: How to build and run Stacker application
---

# Build and Run Stacker

> [!IMPORTANT]
> **Cross-Platform Compatibility Priority**: Tüm geliştirmelerde macOS (Intel/M1), Linux ve Windows uyumluluğu birincil önceliktir. Platforma özel (os-specific) işlemler mutlaka build tag'leri veya runtime kontrolleri ile ayrıştırılmalıdır.

## Development Mode (Quick Test)

// turbo
1. Run the UI server directly:
```bash
go run main.go ui
```

// turbo
2. Or run as tray application:
```bash
go run main.go tray
```

## Production Build

// turbo
1. Build all binaries and app bundles:
```bash
bash build.sh
```

2. Run the macOS app:
```bash
open dist/Stacker-arm64.app
```

## GitHub Release

Automated release workflow via GitHub Actions:

1. Commit changes:
```bash
git add .
git commit -m "chore: release v1.1.0"
```

2. Create and push tag:
```bash
git tag v1.1.0
git push origin v1.1.0
```

This triggers `.github/workflows/release.yml` which builds macOS .app bundles and uploads them to GitHub Releases.

## Build Outputs

- `dist/Stacker-arm64.app` - macOS Apple Silicon app bundle
- `dist/Stacker-amd64.app` - macOS Intel app bundle
- `dist/stacker-linux-amd64` - Linux binary
- `dist/stacker-windows-amd64.exe` - Windows binary

## Data Directory

All configuration and data is stored in:
- macOS: `~/Library/Application Support/Stacker/`
- Linux: `~/.stacker/`

## Troubleshooting

// turbo
If build cache issues occur:
```bash
rm -rf /tmp/go-build-cache
GOCACHE=/tmp/go-build-cache go build -o stacker main.go
```
