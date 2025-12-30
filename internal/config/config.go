package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Domain  string `json:"domain"`
	Port    int    `json:"port"`
	PHPPath string `json:"php_path"`
	Sites   []Site `json:"sites"`
	mu      sync.RWMutex
}

type Site struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var loadedConfig *Config
var configMutex sync.Mutex

func Load(cfgFile string) *Config {
	configMutex.Lock()
	defer configMutex.Unlock()

	if loadedConfig != nil && cfgFile == "" {
		return loadedConfig
	}

	configPath := cfgFile
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		configDir := filepath.Join(home, ".stacker-app")
		os.MkdirAll(configDir, 0755)
		configPath = filepath.Join(configDir, "config.json")
	}

	cfg := &Config{
		Domain:  "*.test",
		Port:    443,
		PHPPath: "/usr/bin/php",
		Sites:   []Site{},
	}

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, cfg)
	}

	loadedConfig = cfg
	return cfg
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".stacker-app")
	os.MkdirAll(configDir, 0755)

	configFile := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

func (c *Config) AddSite(name, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, site := range c.Sites {
		if site.Name == name {
			return
		}
	}
	c.Sites = append(c.Sites, Site{Name: name, Path: path})
}

func (c *Config) RemoveSite(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, site := range c.Sites {
		if site.Name == name {
			c.Sites = append(c.Sites[:i], c.Sites[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) GetSite(name string) *Site {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, site := range c.Sites {
		if site.Name == name {
			return &site
		}
	}
	return nil
}
