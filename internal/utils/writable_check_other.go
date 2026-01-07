//go:build !darwin

package utils

func isWritable(path string) bool {
	return true
}
