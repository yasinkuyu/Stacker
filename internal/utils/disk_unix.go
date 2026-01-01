//go:build !windows

package utils

import (
	"fmt"
	"syscall"
)

func CheckDiskSpace(path string, requiredBytes int64) error {
	var statfs syscall.Statfs_t
	err := syscall.Statfs(path, &statfs)
	if err != nil {
		return nil // Ignore error or return nil to not block if we can't check
	}

	available := uint64(statfs.Bavail) * uint64(statfs.Bsize)
	if uint64(requiredBytes)*2 > available {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d bytes", requiredBytes*2, available)
	}

	return nil
}
