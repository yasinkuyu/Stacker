package php

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/yasinkuyu/Stacker/internal/utils"
)

type PHPVersion struct {
	Version   string `json:"version"`
	Path      string `json:"path"`
	Binary    string `json:"binary"`
	Default   bool   `json:"default"`
	HasXDebug bool   `json:"has_xdebug"`
}

type PHPManager struct {
	versions map[string]*PHPVersion
	mu       sync.RWMutex
	sites    map[string]string // site -> php version
}

func NewPHPManager() *PHPManager {
	return &PHPManager{
		versions: make(map[string]*PHPVersion),
		sites:    make(map[string]string),
	}
}

func (pm *PHPManager) DetectPHPVersions() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	paths := []string{
		"/usr/bin/php",
		"/usr/local/bin/php",
		"/opt/homebrew/bin/php",
		"/opt/homebrew/opt/php*/bin/php",
		filepath.Join(utils.GetStackerDir(), "bin", "php*", "bin", "php"),
	}

	for _, path := range paths {
		files, err := filepath.Glob(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			version, err := pm.getPHPVersion(file)
			if err != nil {
				continue
			}

			pm.versions[version.Version] = version
		}
	}

	// Detect via php -v
	cmd := exec.Command("php", "-v")
	output, err := cmd.CombinedOutput()
	if err == nil {
		version := pm.parsePHPVersionOutput(string(output))
		if version != nil {
			version.Binary = "php"
			pm.versions[version.Version] = version
		}
	}

	return nil
}

func (pm *PHPManager) getPHPVersion(path string) (*PHPVersion, error) {
	cmd := exec.Command(path, "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	version := pm.parsePHPVersionOutput(string(output))
	if version == nil {
		return nil, fmt.Errorf("could not parse version")
	}

	version.Path = path
	version.Binary = path
	version.HasXDebug = pm.checkXDebug(path)

	return version, nil
}

func (pm *PHPManager) parsePHPVersionOutput(output string) *PHPVersion {
	// PHP 8.3.0 (cli)
	re := regexp.MustCompile(`PHP (\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil
	}

	majorMinor := strings.Join(strings.Split(matches[1], ".")[:2], ".")

	return &PHPVersion{
		Version: majorMinor,
	}
}

func (pm *PHPManager) checkXDebug(phpPath string) bool {
	cmd := exec.Command(phpPath, "-m")
	output, _ := cmd.CombinedOutput()
	return strings.Contains(string(output), "xdebug")
}

func (pm *PHPManager) GetVersions() []*PHPVersion {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var versions []*PHPVersion
	for _, version := range pm.versions {
		versions = append(versions, version)
	}
	return versions
}

func (pm *PHPManager) GetVersion(version string) *PHPVersion {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.versions[version]
}

func (pm *PHPManager) SetDefault(version string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, v := range pm.versions {
		v.Default = (v.Version == version)
	}

	return nil
}

func (pm *PHPManager) GetDefault() *PHPVersion {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, version := range pm.versions {
		if version.Default {
			return version
		}
	}

	if len(pm.versions) > 0 {
		for _, version := range pm.versions {
			return version
		}
	}

	return nil
}

func (pm *PHPManager) PinSite(siteName, phpVersion string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.versions[phpVersion]; !ok {
		return fmt.Errorf("PHP version %s not found", phpVersion)
	}

	pm.sites[siteName] = phpVersion
	return nil
}

func (pm *PHPManager) UnpinSite(siteName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.sites, siteName)
}

func (pm *PHPManager) GetSitePHP(siteName string) *PHPVersion {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if version, ok := pm.sites[siteName]; ok {
		return pm.versions[version]
	}

	return pm.GetDefault()
}

func (pm *PHPManager) FormatVersions() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.versions) == 0 {
		return "No PHP versions detected. Run 'php --detect' to scan."
	}

	var output strings.Builder
	output.WriteString("Available PHP versions:\n")

	for _, version := range pm.versions {
		prefix := "  "
		if version.Default {
			prefix = "* "
		}

		xdebug := ""
		if version.HasXDebug {
			xdebug = " (XDebug)"
		}

		output.WriteString(fmt.Sprintf("%s%s%s - %s\n", prefix, version.Version, xdebug, version.Binary))
	}

	return output.String()
}

func (pm *PHPManager) ExecutePHPCommand(version string, args ...string) error {
	php := pm.GetVersion(version)
	if php == nil {
		php = pm.GetDefault()
	}

	if php == nil {
		return fmt.Errorf("PHP version not found")
	}

	cmd := exec.Command(php.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
