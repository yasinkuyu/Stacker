//go:build windows

package utils

func CheckDiskSpace(path string, requiredBytes int64) error {
	// For Windows, we can implement GetDiskFreeSpaceEx if needed.
	// For now, return nil to not block build.
	return nil
}
