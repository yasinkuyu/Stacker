package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

const UpdateJSONURL = "https://raw.githubusercontent.com/yasinkuyu/Stacker/main/update.json"

// RemoteConfig represents the structure of update.json
type RemoteConfig struct {
	Stacker  StackerInfo           `json:"stacker"`
	Services map[string]ServiceDef `json:"services"`
}

type StackerInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"releaseDate"`
	DownloadURL string `json:"downloadUrl"`
	Changelog   string `json:"changelog"`
}

type ServiceDef struct {
	Versions map[string]string `json:"versions"`
	Sources  []string          `json:"sources"`
}

// ServiceVersion represents an available service version for installation
type ServiceVersion struct {
	Type      string
	Version   string
	FullVer   string
	Available bool
	Arch      string
	URL       string
}

var cachedConfig *RemoteConfig

// FetchRemoteConfig fetches update.json from GitHub
func FetchRemoteConfig() (*RemoteConfig, error) {
	if cachedConfig != nil {
		return cachedConfig, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(UpdateJSONURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch update.json: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var config RemoteConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}

	cachedConfig = &config
	return &config, nil
}

// GetFullVersion returns the full version for a short version of a service
func GetFullVersion(serviceType, shortVersion string) string {
	config, err := FetchRemoteConfig()
	if err != nil {
		// Fallback to embedded defaults
		for _, v := range GetDefaultVersions(serviceType) {
			if v.Version == shortVersion {
				return v.FullVer
			}
		}
		return shortVersion
	}

	service, ok := config.Services[serviceType]
	if !ok {
		return shortVersion
	}

	fullVer, ok := service.Versions[shortVersion]
	if !ok {
		return shortVersion
	}

	return fullVer
}

// GetDownloadURL returns the download URL for a specific version of a service
func GetDownloadURL(serviceType, shortVersion string) string {
	versions := GetAvailableVersions(serviceType)
	for _, v := range versions {
		if v.Version == shortVersion {
			return v.URL
		}
	}
	return ""
}

// GetAvailableVersions returns all available versions for a service type
func GetAvailableVersions(serviceType string) []ServiceVersion {
	config, err := FetchRemoteConfig()
	if err != nil {
		// Fallback to embedded defaults
		return GetDefaultVersions(serviceType)
	}

	service, ok := config.Services[serviceType]
	if !ok {
		return nil
	}

	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "arm64"
	} else if arch == "amd64" {
		arch = "x86_64"
	}

	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}

	var versions []ServiceVersion
	for shortVer, fullVer := range service.Versions {
		url := buildDownloadURL(serviceType, osName, arch, shortVer, fullVer, service.Sources)
		versions = append(versions, ServiceVersion{
			Type:      serviceType,
			Version:   shortVer,
			FullVer:   fullVer,
			Available: true,
			Arch:      arch,
			URL:       url,
		})
	}

	return versions
}

// buildDownloadURL constructs the download URL based on service type
func buildDownloadURL(serviceType, osName, arch, shortVer, fullVer string, sources []string) string {
	source := ""
	if len(sources) > 0 {
		source = sources[0]
	}

	switch serviceType {
	case "php":
		archName := arch
		if arch == "arm64" {
			archName = "aarch64"
		}
		return fmt.Sprintf("https://dl.static-php.dev/static-php-cli/common/php-%s-cli-%s-%s.tar.gz", fullVer, osName, archName)

	case "mariadb":
		if osName == "macos" {
			if arch == "arm64" {
				return fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/bintar-mac-macos14-arm64/mariadb-%s-mac-macos14-arm64.tar.gz", fullVer, fullVer)
			}
			return fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/bintar-mac-macos14-x86_64/mariadb-%s-mac-macos14-x86_64.tar.gz", fullVer, fullVer)
		}
		if arch == "arm64" {
			return fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/bintar-linux-systemd-aarch64/mariadb-%s-linux-systemd-aarch64.tar.gz", fullVer, fullVer)
		}
		return fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/bintar-linux-systemd-x86_64/mariadb-%s-linux-systemd-x86_64.tar.gz", fullVer, fullVer)

	case "mysql":
		if osName == "macos" {
			if arch == "arm64" {
				return fmt.Sprintf("https://dev.mysql.com/get/Downloads/MySQL-%s/mysql-%s-macos14-arm64.tar.gz", shortVer, fullVer)
			}
			return fmt.Sprintf("https://dev.mysql.com/get/Downloads/MySQL-%s/mysql-%s-macos14-x86_64.tar.gz", shortVer, fullVer)
		}
		if arch == "arm64" {
			return fmt.Sprintf("https://dev.mysql.com/get/Downloads/MySQL-%s/mysql-%s-linux-glibc2.28-aarch64.tar.xz", shortVer, fullVer)
		}
		return fmt.Sprintf("https://dev.mysql.com/get/Downloads/MySQL-%s/mysql-%s-linux-glibc2.28-x86_64.tar.xz", shortVer, fullVer)

	case "redis":
		return fmt.Sprintf("https://github.com/redis/redis/archive/refs/tags/%s.tar.gz", fullVer)

	case "nginx":
		return fmt.Sprintf("https://nginx.org/download/nginx-%s.tar.gz", fullVer)

	case "apache":
		return fmt.Sprintf("https://archive.apache.org/dist/httpd/httpd-%s.tar.gz", fullVer)

	case "nodejs":
		if osName == "macos" {
			if arch == "arm64" {
				return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-darwin-arm64.tar.gz", fullVer, fullVer)
			}
			return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-darwin-x64.tar.gz", fullVer, fullVer)
		}
		if arch == "arm64" {
			return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-linux-arm64.tar.xz", fullVer, fullVer)
		}
		return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-linux-x64.tar.xz", fullVer, fullVer)

	case "composer":
		return fmt.Sprintf("https://getcomposer.org/download/%s/composer.phar", fullVer)

	default:
		return fmt.Sprintf("%s/%s", source, fullVer)
	}
}

func GetDefaultVersions(serviceType string) []ServiceVersion {
	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "arm64"
	} else if arch == "amd64" {
		arch = "x86_64"
	}

	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}

	defaults := map[string]map[string]string{
		"php":      {"8.4": "8.4.16", "8.3": "8.3.29", "8.2": "8.2.30", "8.1": "8.1.34", "8.0": "8.0.30"},
		"mariadb":  {"11.4": "11.4.5", "10.11": "10.11.11", "10.6": "10.6.21"},
		"mysql":    {"8.0": "8.0.40", "5.7": "5.7.44"},
		"redis":    {"7.4": "7.4.2", "7.2": "7.2.6", "7.0": "7.0.15"},
		"nginx":    {"1.27": "1.27.3", "1.26": "1.26.2", "1.24": "1.24.0"},
		"apache":   {"2.4": "2.4.62"},
		"nodejs":   {"22": "22.12.0", "20": "20.18.1", "18": "18.20.5"},
		"composer": {"2": "2.8.4"},
	}

	var versions []ServiceVersion
	if svc, ok := defaults[serviceType]; ok {
		for shortVer, fullVer := range svc {
			url := buildDownloadURL(serviceType, osName, arch, shortVer, fullVer, nil)
			versions = append(versions, ServiceVersion{
				Type:      serviceType,
				Version:   shortVer,
				FullVer:   fullVer,
				Available: true,
				Arch:      arch,
				URL:       url,
			})
		}
	}

	return versions
}
