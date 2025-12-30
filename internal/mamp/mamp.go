package mamp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MAMPConfig struct {
	ApachePort int            `json:"apache_port"`
	MySQLPort  int            `json:"mysql_port"`
	Hosts      []MAMPHost     `json:"hosts"`
	Databases  []MAMPDatabase `json:"databases"`
}

type MAMPHost struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Host string `json:"host"`
}

type MAMPDatabase struct {
	Name string `json:"name"`
	User string `json:"user"`
	Pass string `json:"password"`
}

func NewMAMPImporter() *MAMPImporter {
	return &MAMPImporter{}
}

type MAMPImporter struct{}

func (mi *MAMPImporter) GetMAMPPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	paths := []string{
		filepath.Join(home, "Applications/MAMP"),
		filepath.Join("/Applications", "MAMP"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("MAMP not found")
}

func (mi *MAMPImporter) ImportSites(mampPath string) ([]MAMPHost, error) {
	hostsFile := filepath.Join(mampPath, "conf/apache/hosts")
	if _, err := os.Stat(hostsFile); err != nil {
		return nil, fmt.Errorf("MAMP hosts file not found")
	}

	data, err := os.ReadFile(hostsFile)
	if err != nil {
		return nil, err
	}

	var config MAMPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MAMP config: %w", err)
	}

	return config.Hosts, nil
}

func (mi *MAMPImporter) ImportDatabases(mampPath string) ([]MAMPDatabase, error) {
	hostsFile := filepath.Join(mampPath, "conf/apache/hosts")
	if _, err := os.Stat(hostsFile); err != nil {
		return nil, fmt.Errorf("MAMP hosts file not found")
	}

	data, err := os.ReadFile(hostsFile)
	if err != nil {
		return nil, err
	}

	var config MAMPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MAMP config: %w", err)
	}

	return config.Databases, nil
}

func (mi *MAMPImporter) ScanMAMPProjects(basePath string) ([]MAMPHost, error) {
	var hosts []MAMPHost

	htdocs := filepath.Join(basePath, "htdocs")
	if _, err := os.Stat(htdocs); err != nil {
		return nil, fmt.Errorf("htdocs folder not found: %s", htdocs)
	}

	entries, err := os.ReadDir(htdocs)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			projectPath := filepath.Join(htdocs, entry.Name())
			publicPath := filepath.Join(projectPath, "public")

			if _, err := os.Stat(publicPath); err == nil {
				hosts = append(hosts, MAMPHost{
					Name: entry.Name(),
					Path: projectPath,
					Host: fmt.Sprintf("%s.local", entry.Name()),
				})
			}
		}
	}

	return hosts, nil
}
