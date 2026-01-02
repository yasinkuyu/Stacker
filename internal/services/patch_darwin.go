package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// patchHomebrewBottle replaces @@HOMEBREW_PREFIX@@ placeholders in binaries
// with the actual Homebrew prefix on the system.
func (sm *ServiceManager) patchHomebrewBottle(installDir string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	fmt.Printf("🔧 Patching Homebrew bottle in %s...\n", installDir)

	// Determine Homebrew prefix
	brewPrefix := "/usr/local"
	if runtime.GOARCH == "arm64" {
		brewPrefix = "/opt/homebrew"
	}

    // Check if user has custom brew location (optional check)
    if _, err := os.Stat(brewPrefix); os.IsNotExist(err) {
        fmt.Printf("⚠️ Homebrew prefix %s not found. Patching might fail or libraries won't be found.\n", brewPrefix)
    }

	// Walk through the directory to find files to patch
	err := filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only check executable files and dylibs/so
		if isMachO(path) {
			if err := patchBinary(path, brewPrefix); err != nil {
				fmt.Printf("⚠️ Failed to patch %s: %v\n", path, err)
                // Don't fail the entire process, just log warning
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk install dir: %w", err)
	}

	fmt.Println("✅ Patching complete.")
	return nil
}

// isMachO checks if a file is a Mach-O binary (executable or library)
func isMachO(path string) bool {
	// Simple check by extension or permission is not enough.
	// We can use 'file' command or read magic bytes.
    // Reading magic bytes is faster and safer.
    f, err := os.Open(path)
    if err != nil {
        return false
    }
    defer f.Close()

    b := make([]byte, 4)
    if _, err := f.Read(b); err != nil {
        return false
    }

    // Mach-O magic numbers
    // 32-bit: 0xfeedface or 0xcefaedfe
    // 64-bit: 0xfeedfacf or 0xcffaedfe
    // Universal: 0xcafebabe or 0xbebafeca
    if (b[0] == 0xfe && b[1] == 0xed && b[2] == 0xfa && (b[3] == 0xce || b[3] == 0xcf)) || // Native
       (b[0] == 0xce && b[1] == 0xfa && b[2] == 0xed && b[3] == 0xfe) || // Reverse 32
       (b[0] == 0xcf && b[1] == 0xfa && b[2] == 0xed && b[3] == 0xfe) || // Reverse 64
       (b[0] == 0xca && b[1] == 0xfe && b[2] == 0xba && b[3] == 0xbe) { // Universal
        return true
    }
    return false
}

func patchBinary(path string, brewPrefix string) error {
	// Get list of dynamic libraries
	cmd := exec.Command("otool", "-L", path)
	output, err := cmd.Output()
	if err != nil {
		return nil // Not a valid binary or otool failed
	}

	lines := strings.Split(string(output), "\n")
	needsPatch := false
    
    // Check if the file itself is a dylib that needs ID update
    idCmd := exec.Command("otool", "-D", path)
    if idOut, err := idCmd.Output(); err == nil {
        idStr := strings.TrimSpace(string(idOut))
        // recursive output usually contains filename then ID
        // simplistically check content
        if strings.Contains(idStr, "@@HOMEBREW_PREFIX@@") {
             // Extract ID (it's usually the second line of otool -D output)
             lines := strings.Split(idStr, "\n")
             if len(lines) >= 2 {
                 oldId := strings.TrimSpace(lines[1])
                 if strings.Contains(oldId, "@@HOMEBREW_PREFIX@@") {
                     newId := strings.ReplaceAll(oldId, "@@HOMEBREW_PREFIX@@", brewPrefix)
                     fmt.Printf("🔧 Patching ID: %s -> %s\n", oldId, newId)
                     if err := exec.Command("install_name_tool", "-id", newId, path).Run(); err != nil {
                         fmt.Printf("Failed to change ID for %s: %v\n", path, err)
                     }
                 }
             }
        }
    }

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Format is: /path/to/lib (compatibility version ..., current version ...)
		if strings.Contains(line, "@@HOMEBREW_PREFIX@@") {
			parts := strings.Split(line, " ")
			if len(parts) > 0 {
				oldPath := parts[0]
				newPath := strings.ReplaceAll(oldPath, "@@HOMEBREW_PREFIX@@", brewPrefix)
                
                // Only patch if it changed
                if oldPath != newPath {
                    fmt.Printf("🔧 Patching %s: %s -> %s\n", filepath.Base(path), oldPath, newPath)
                    changeCmd := exec.Command("install_name_tool", "-change", oldPath, newPath, path)
                    if out, err := changeCmd.CombinedOutput(); err != nil {
                        return fmt.Errorf("install_name_tool failed: %v, output: %s", err, string(out))
                    }
                    needsPatch = true
                }
			}
		}
	}

    // Attempt to codesign if patched (ad-hoc signing for arm64)
	if needsPatch && runtime.GOARCH == "arm64" {
		exec.Command("codesign", "--force", "--sign", "-", path).Run()
	}

	return nil
}
