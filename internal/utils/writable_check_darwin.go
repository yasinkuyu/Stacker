//go:build darwin

package utils

import (
	"path/filepath"
	"syscall"
)

func isWritable(path string) bool {
	return syscall.Access(filepath.Dir(path), 2) == nil
}
