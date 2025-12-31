package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetStackerDir returns the Stacker application data directory
// On macOS: ~/Library/Application Support/Stacker (or inside .app bundle for portable)
// This keeps all data self-contained like MAMP
func GetStackerDir() string {
	// First check if running from .app bundle
	execPath, err := os.Executable()
	if err == nil {
		// Check if inside .app bundle
		if strings.Contains(execPath, ".app/Contents/MacOS") {
			// Use .app/Contents/Resources for self-contained mode
			appDir := filepath.Dir(filepath.Dir(execPath))
			return filepath.Join(appDir, "Resources", "Stacker")
		}
	}

	// Fallback to Application Support for development
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Stacker")
	}
	// Linux fallback
	return filepath.Join(home, ".stacker")
}
