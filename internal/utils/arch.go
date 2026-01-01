package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type ArchInfo struct {
	GOOS        string
	GOARCH      string
	NativeArch  string
	Universal   bool
	Rosetta     bool
	ArchAliases []string
}

func GetArchInfo() ArchInfo {
	info := ArchInfo{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
	}

	switch runtime.GOARCH {
	case "amd64":
		info.NativeArch = "x86_64"
		info.ArchAliases = []string{"x86_64", "amd64", "x64"}
	case "arm64":
		info.NativeArch = "arm64"
		info.ArchAliases = []string{"arm64", "aarch64"}
	default:
		info.NativeArch = runtime.GOARCH
		info.ArchAliases = []string{runtime.GOARCH}
	}

	if runtime.GOOS == "darwin" {
		info.Universal = checkUniversalBinarySupport()
		info.Rosetta = checkRosetta()
	}

	return info
}

func checkRosetta() bool {
	if runtime.GOARCH != "arm64" {
		return false
	}

	out, err := exec.Command("sysctl", "-in", "sysctl.proc_translated").Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}

func checkUniversalBinarySupport() bool {
	if runtime.GOARCH == "arm64" || checkRosetta() {
		return true
	}
	return false
}

func IsBinaryCompatible(binaryArch string, info ArchInfo) bool {
	if binaryArch == "all" || binaryArch == "any" {
		return true
	}

	if binaryArch == "universal" || binaryArch == "universal2" {
		return info.GOOS == "darwin" && info.Universal
	}

	if binaryArch == info.NativeArch {
		return true
	}

	for _, alias := range info.ArchAliases {
		if binaryArch == alias {
			return true
		}
	}

	if info.Rosetta && (binaryArch == "x86_64" || binaryArch == "amd64") {
		return true
	}

	return false
}

func VerifyBinaryChecksum(path string, expectedHash string) error {
	if expectedHash == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func VerifyDownloadedBinary(binaryPath string, expectedArch string, info ArchInfo) error {
	file, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 4)
	if _, err := file.Read(header); err != nil {
		return err
	}

	actualArch := detectBinaryArch(header)
	if !IsBinaryCompatible(expectedArch, info) && !IsBinaryCompatible(actualArch, info) {
		return fmt.Errorf("binary arch %s not compatible with system %s", expectedArch, info.NativeArch)
	}

	return nil
}

func detectBinaryArch(header []byte) string {
	if len(header) < 4 {
		return "unknown"
	}

	if header[0] == 0xcf && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe {
		return "universal"
	}

	if header[0] == 0xfe && header[1] == 0xed && header[2] == 0xfa && header[3] == 0xcf {
		return "universal"
	}

	if header[0] == 0xfe && header[1] == 0xed && header[2] == 0xfa && header[3] == 0xcf {
		if len(header) >= 8 {
			cpuType := header[7]
			if cpuType == 0x01 {
				return "x86_64"
			}
			return "arm64"
		}
	}

	if header[0] == 0x7f && header[1] == 0x45 && header[2] == 0x4c && header[3] == 0x46 {
		return "linux"
	}

	return "unknown"
}
