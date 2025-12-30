package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type StackerConfig struct {
	PHP      string                 `yaml:"php"`
	Services []ServiceConfig        `yaml:"services"`
	Forge    *ForgeConfig           `yaml:"forge,omitempty"`
	Env      map[string]string      `yaml:"env,omitempty"`
	Extra    map[string]interface{} `yaml:",inline"`
}

type ServiceConfig struct {
	Type    string `yaml:"type"`
	Version string `yaml:"version,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

type ForgeConfig struct {
	ServerID string `yaml:"server_id"`
	SiteID   string `yaml:"site_id"`
}

func LoadStackerYaml(projectPath string) (*StackerConfig, error) {
	configFile := filepath.Join(projectPath, "stacker.yml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return &StackerConfig{}, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read stacker.yml: %w", err)
	}

	var config StackerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse stacker.yml: %w", err)
	}

	return &config, nil
}

func CreateStackerYaml(projectPath string, phpVersion string) error {
	config := StackerConfig{
		PHP: phpVersion,
		Services: []ServiceConfig{
			{Type: "mysql", Version: "8.0", Port: 3306},
			{Type: "redis", Port: 6379},
		},
		Env: map[string]string{
			"APP_ENV":   "local",
			"APP_DEBUG": "true",
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	configFile := filepath.Join(projectPath, "stacker.yml")
	return os.WriteFile(configFile, data, 0644)
}
