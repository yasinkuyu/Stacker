package services

import (
	"os/exec"
	"regexp"
)

// GetSystemVersion checks if a service is installed on the system and returns its version
func GetSystemVersion(svcType string) (string, error) {
	var cmd *exec.Cmd
	var versionArg string

	switch svcType {
	case "composer":
		cmd = exec.Command("composer", "--version")
	case "nginx":
		cmd = exec.Command("nginx", "-v")
	case "apache":
		cmd = exec.Command("httpd", "-v") // macOS default
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("apachectl", "-v")
		}
	case "mysql":
		cmd = exec.Command("mysql", "--version")
	case "mariadb":
		cmd = exec.Command("mariadb", "--version")
	case "php":
		cmd = exec.Command("php", "-v")
	case "nodejs":
		cmd = exec.Command("node", "-v")
	case "redis":
		cmd = exec.Command("redis-server", "--version")
	default:
		return "", nil
	}

	// Try common paths if not in PATH
	if cmd.Err != nil {
		// Common macOS paths
		paths := []string{
			"/opt/homebrew/bin/" + cmd.Path,
			"/opt/homebrew/sbin/" + cmd.Path,
			"/usr/local/bin/" + cmd.Path,
			"/usr/local/sbin/" + cmd.Path,
		}
		for _, p := range paths {
			if pathCmd := exec.Command(p, versionArg); pathCmd.Run() == nil {
				cmd = pathCmd
				break
			}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	outStr := string(output)

	// Parse version based on type
	switch svcType {
	case "composer":
		// Composer version 2.7.6 ...
		re := regexp.MustCompile(`Composer version ([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "nginx":
		// nginx version: nginx/1.27.3
		re := regexp.MustCompile(`nginx/([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "apache":
		// Server version: Apache/2.4.56 (Unix)
		re := regexp.MustCompile(`Apache/([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "mysql", "mariadb":
		// mysql  Ver 8.0.33 for macos13.3 on arm64 (Homebrew)
		// mariadb  Ver 15.1 Distrib 10.6.12-MariaDB
		re := regexp.MustCompile(`Ver ([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
		reDistrib := regexp.MustCompile(`Distrib ([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := reDistrib.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "php":
		// PHP 8.2.8 (cli) (built: Jul  6 2023 10:59:16) (NTS)
		re := regexp.MustCompile(`PHP ([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "nodejs":
		// v20.5.0
		re := regexp.MustCompile(`v([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	case "redis":
		// Redis server v=7.0.12 sha=00000000:0 malloc=libc bits=64 build=...
		re := regexp.MustCompile(`v=([0-9]+\.[0-9]+\.[0-9]+)`)
		if matches := re.FindStringSubmatch(outStr); len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", nil
}
