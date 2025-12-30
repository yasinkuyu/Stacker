package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Preferences struct {
	Theme     string `json:"theme"` // "light" or "dark"
	AutoStart bool   `json:"autoStart"`
	Port      int    `json:"port"`
	ShowTray  bool   `json:"showTray"`
}

var prefs *Preferences

func LoadPreferences() *Preferences {
	if prefs != nil {
		return prefs
	}

	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".stacker-app")
	os.MkdirAll(configDir, 0755)

	prefsPath := filepath.Join(configDir, "preferences.json")

	if data, err := os.ReadFile(prefsPath); err == nil {
		prefs = &Preferences{}
		json.Unmarshal(data, prefs)
	} else {
		prefs = &Preferences{
			Theme:     "dark",
			AutoStart: false,
			Port:      8080,
			ShowTray:  true,
		}
	}

	return prefs
}

func (p *Preferences) Save() error {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".stacker-app")
	os.MkdirAll(configDir, 0755)

	prefsPath := filepath.Join(configDir, "preferences.json")

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(prefsPath, data, 0644)
}

func GetPreferences() *Preferences {
	return LoadPreferences()
}
