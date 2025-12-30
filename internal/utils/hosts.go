package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"runtime"
)

const herdMarker = "# stacker-app"

func GetHostsPath() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\Windows\System32\drivers\etc\hosts`
	case "darwin", "linux":
		return "/etc/hosts"
	default:
		return "/etc/hosts"
	}
}

func AddToHosts(hostname string) error {
	hostsPath := GetHostsPath()

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "127.0.0.1\t"+hostname+" "+herdMarker {
			return nil
		}
	}

	entry := fmt.Sprintf("127.0.0.1\t%s %s\n", hostname, herdMarker)

	f, err := os.OpenFile(hostsPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open hosts file: %w (try running with sudo)", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w", err)
	}

	return nil
}

func RemoveFromHosts(hostname string) error {
	hostsPath := GetHostsPath()

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	var newContent []byte
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "127.0.0.1\t"+hostname+" "+herdMarker || line == "127.0.0.1 "+hostname+" "+herdMarker {
			continue
		}
		newContent = append(newContent, line...)
		newContent = append(newContent, '\n')
	}

	if err := os.WriteFile(hostsPath, newContent, 0644); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w", err)
	}

	return nil
}
