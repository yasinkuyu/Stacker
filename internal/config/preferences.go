package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Preferences struct {
	Theme             string   `json:"theme"`
	AutoStart         bool     `json:"autoStart"`
	AutoStartServices bool     `json:"autoStartServices"`
	ActiveServices    []string `json:"activeServices"`
	ShowTray          bool     `json:"showTray"`
	Port              int      `json:"port"`
	SlimMode          bool     `json:"slimMode"`
	DomainExtension   string   `json:"domainExtension"`
	ApachePort        int      `json:"apachePort"`
	NginxPort         int      `json:"nginxPort"`
	MySQLPort         int      `json:"mysqlPort"`
	Language          string   `json:"language"`
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

		// Defaults for new fields
		if prefs.Language == "" {
			prefs.Language = "en"
		}
		if prefs.DomainExtension == "" {
			prefs.DomainExtension = "local"
		}

		// Auto-migrate from 8080 to 9999
		if prefs.Port == 8080 {
			prefs.Port = 9999
			prefs.AutoStartServices = true
			prefs.Save()
		}
	} else {
		prefs = &Preferences{
			Theme:             "dark",
			AutoStart:         false,
			AutoStartServices: true,
			Port:              9999,
			ShowTray:          true,
			SlimMode:          false,
			DomainExtension:   "local",
			Language:          "en",
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
