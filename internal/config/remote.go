package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
)

const UpdateJSONURL = "https://raw.githubusercontent.com/yasinkuyu/Stacker/main/update.json"

var (
	cachedConfig *RemoteConfig
	cacheMutex   sync.RWMutex
	lastFetch    time.Time
	etag         string
)

type RemoteConfig struct {
	Meta     MetaInfo              `json:"meta"`
	Stacker  StackerInfo           `json:"stacker"`
	Services map[string]ServiceDef `json:"services"`
}

type MetaInfo struct {
	Version     string `json:"version"`
	LastUpdated string `json:"lastUpdated"`
	TTL         int    `json:"ttl"`
}

type ServiceDef struct {
	Description  string                `json:"description"`
	URLTemplate  string                `json:"urlTemplate,omitempty"`
	ChecksumType string                `json:"checksumType,omitempty"`
	Versions     map[string]VersionDef `json:"versions"`
	Sources      []string              `json:"sources"`
}

type VersionDef struct {
	FullVersion string                 `json:"fullVersion"`
	ReleaseDate string                 `json:"releaseDate"`
	Artifacts   map[string]ArtifactDef `json:"artifacts"`
}

type ArtifactDef struct {
	URL       string   `json:"url"`
	Checksum  string   `json:"checksum"`
	Size      int64    `json:"size"`
	Mirrors   []string `json:"mirrors,omitempty"`
	Preferred bool     `json:"preferred,omitempty"`
}

type StackerInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"releaseDate"`
	DownloadURL string `json:"downloadUrl"`
	Changelog   string `json:"changelog"`
}

// ServiceVersion represents an available service version for installation
type ServiceVersion struct {
	Type            string   `json:"type"`
	Version         string   `json:"version"`
	FullVer         string   `json:"fullVer"`
	Available       bool     `json:"available"`
	Installed       bool     `json:"installed"`
	SystemInstalled bool     `json:"system_installed"`
	Arch            string   `json:"arch"`
	Platform        string   `json:"platform"`
	URL             string   `json:"url"`
	Checksum        string   `json:"checksum"`
	Size            int64    `json:"size"`
	Mirrors         []string `json:"mirrors,omitempty"`
	Preferred       bool     `json:"preferred"`
}

// FetchRemoteConfig fetches update.json from GitHub with caching and ETag support
func FetchRemoteConfig() (*RemoteConfig, error) {
	// Check for local update.json for development
	if _, err := os.Stat("update.json"); err == nil {
		data, err := os.ReadFile("update.json")
		if err == nil {
			var config RemoteConfig
			if err := json.Unmarshal(data, &config); err == nil {
				return &config, nil
			}
		}
	}

	cacheMutex.RLock()
	if cachedConfig != nil && time.Since(lastFetch) < 10*time.Minute {
		defer cacheMutex.RUnlock()
		return cachedConfig, nil
	}
	currentEtag := etag
	cacheMutex.RUnlock()

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", UpdateJSONURL, nil)

	if currentEtag != "" {
		req.Header.Set("If-None-Match", currentEtag)
	}

	resp, err := client.Do(req)
	if err != nil {
		cacheMutex.RLock()
		defer cacheMutex.RUnlock()
		if cachedConfig != nil {
			return cachedConfig, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		cacheMutex.RLock()
		defer cacheMutex.RUnlock()
		if cachedConfig != nil {
			return cachedConfig, nil
		}
		// If 304 but no cache, force a fresh fetch
		req.Header.Del("If-None-Match")
		resp, err = client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch update.json (304 with no cache)")
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch update.json: status %d", resp.StatusCode)
	}

	newEtag := resp.Header.Get("ETag")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var newConfig RemoteConfig
	if err := json.Unmarshal(body, &newConfig); err != nil {
		return nil, err
	}

	cacheMutex.Lock()
	cachedConfig = &newConfig
	lastFetch = time.Now()
	etag = newEtag
	cacheMutex.Unlock()

	return &newConfig, nil
}

// GetFullVersion returns the full version for a short version of a service
func GetFullVersion(serviceType, shortVersion string) string {
	remoteCfg, err := FetchRemoteConfig()
	if err != nil || remoteCfg == nil {
		// Fallback to embedded defaults
		for _, v := range GetDefaultVersions(serviceType) {
			if v.Version == shortVersion {
				return v.FullVer
			}
		}
		return shortVersion
	}

	service, ok := remoteCfg.Services[serviceType]
	if !ok {
		return shortVersion
	}

	versionDef, ok := service.Versions[shortVersion]
	if !ok {
		return shortVersion
	}

	return versionDef.FullVersion
}

// GetDownloadURL returns the download URL for a specific version of a service
func GetDownloadURL(serviceType, shortVersion string) string {
	versions := GetAvailableVersions(serviceType, "")
	for _, v := range versions {
		if v.Version == shortVersion {
			return v.URL
		}
	}
	return ""
}

// GetAvailableVersions returns all available versions for a service type
func GetAvailableVersions(serviceType string, platform string) []ServiceVersion {
	if platform == "" {
		arch := runtime.GOARCH
		if arch == "arm64" {
			arch = "arm64"
		} else if arch == "amd64" {
			arch = "x86_64"
		}

		osName := runtime.GOOS
		if osName == "darwin" {
			// Keep darwin for consistency with artifacts keys
		} else if osName == "linux" {
			osName = "linux"
		}

		platform = fmt.Sprintf("%s-%s", osName, arch)
	}
	remoteCfg, err := FetchRemoteConfig()
	if err != nil || remoteCfg == nil {
		// Fallback to embedded defaults
		return GetDefaultVersions(serviceType)
	}

	// If serviceType is empty, return versions for all services
	var serviceTypes []string
	if serviceType == "" {
		for k := range remoteCfg.Services {
			if k == "php" {
				continue
			}
			serviceTypes = append(serviceTypes, k)
		}
	} else {
		serviceTypes = []string{serviceType}
	}

	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "arm64"
	} else if arch == "amd64" {
		arch = "x86_64"
	}

	osName := runtime.GOOS
	if osName == "darwin" {
		// Keep darwin
	} else if osName == "linux" {
		osName = "linux"
	}

	if platform == "" {
		platform = fmt.Sprintf("%s-%s", osName, arch)
	}

	var versions []ServiceVersion
	for _, st := range serviceTypes {
		service, ok := remoteCfg.Services[st]
		if !ok {
			continue
		}

		for shortVer, versionDef := range service.Versions {
			for artifactKey, artifact := range versionDef.Artifacts {
				if artifactKey == "all" || artifactKey == platform || artifactKey == osName {
					versions = append(versions, ServiceVersion{
						Type:      st,
						Version:   shortVer,
						FullVer:   versionDef.FullVersion,
						Available: true,
						Arch:      artifactKey,
						Platform:  platform,
						URL:       artifact.URL,
						Checksum:  artifact.Checksum,
						Size:      artifact.Size,
						Mirrors:   artifact.Mirrors,
						Preferred: artifact.Preferred,
					})
				}
			}
		}
	}

	// Sort by preferred flag first
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Preferred != versions[j].Preferred {
			return versions[i].Preferred
		}
		// Then by type
		if versions[i].Type != versions[j].Type {
			return versions[i].Type < versions[j].Type
		}
		// Then by version descending
		return versions[i].Version > versions[j].Version
	})

	return versions
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
		// Keep darwin
	} else if osName == "linux" {
		osName = "linux"
	}

	platform := fmt.Sprintf("%s-%s", osName, arch)

	defaults := map[string]map[string]string{
		"php":      {"8.4": "8.4.16", "8.3": "8.3.29", "8.2": "8.2.30", "8.1": "8.1.34", "8.0": "8.0.30"},
		"mariadb":  {"11.4": "11.4.5", "10.11": "10.11.11", "10.6": "10.6.21"},
		"mysql":    {"8.0": "8.0.40", "5.7": "5.7.44"},
		"redis":    {"7.4": "7.4.2", "7.2": "7.2.6", "7.0": "7.0.15"},
		"nginx":    {"1.27": "1.27.3"},
		"apache":   {"2.4": "2.4.62"},
		"nodejs":   {"22": "22.12.0", "20": "20.18.1", "18": "18.20.5"},
		"composer": {"2": "2.8.4"},
	}

	var serviceTypes []string
	if serviceType == "" {
		for k := range defaults {
			if k == "php" {
				continue
			}
			serviceTypes = append(serviceTypes, k)
		}
	} else {
		serviceTypes = []string{serviceType}
	}

	var versions []ServiceVersion
	for _, st := range serviceTypes {
		if svc, ok := defaults[st]; ok {
			for shortVer, fullVer := range svc {
				versions = append(versions, ServiceVersion{
					Type:      st,
					Version:   shortVer,
					FullVer:   fullVer,
					Available: true,
					Arch:      platform,
					Platform:  platform,
					URL:       "",
				})
			}
		}
	}

	return versions
}
