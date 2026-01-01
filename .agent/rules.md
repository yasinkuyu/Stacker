# Global Rules for Stacker

## 1. Cross-Platform Compatibility
- **Primary Goal**: All features must work on macOS (ARM64/AMD64), Linux (AMD64), and Windows (AMD64).
- **Subsystem Isolation**: Platform-specific logic (filesystem paths, syscalls, process management) must be isolated using Go build tags (`//go:build`) or runtime checks (`runtime.GOOS`).
- **Dependencies**: Prefer pure-Go dependencies or those that are well-supported across all target platforms.
- **Build Verification**: Every significant change must be verified using `bash build.sh` or targeted builds for all platforms.

## 2. Service Management
- All services must follow the controlled lifecycle managed by `ServiceManager`.
- Configuration files must be generated dynamically to ensure portability.

## 3. UI Aesthetics
- Maintain a premium, first-glance "WOW" design across all platforms.
- Use consistent typography and color palettes as defined in the design system.
