package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/yasinkuyu/Stacker/internal/utils"
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

	configDir := utils.GetStackerDir()
	os.MkdirAll(configDir, 0755)

	prefsPath := filepath.Join(configDir, "preferences.json")

	if data, err := os.ReadFile(prefsPath); err == nil {
		prefs = &Preferences{}
		json.Unmarshal(data, prefs)

		// Defaults for new fields
		shouldSave := false
		if prefs.Language == "" {
			prefs.Language = "en"
			shouldSave = true
		}
		if prefs.DomainExtension == "" {
			prefs.DomainExtension = "local"
			shouldSave = true
		}
		if prefs.ApachePort == 0 {
			prefs.ApachePort = 8080
			shouldSave = true
		}
		if prefs.NginxPort == 0 {
			prefs.NginxPort = 80
			shouldSave = true
		}
		if prefs.MySQLPort == 0 {
			prefs.MySQLPort = 3306
			shouldSave = true
		}

		// Auto-migrate from 8080 to 9999
		if prefs.Port == 8080 {
			prefs.Port = 9999
			prefs.AutoStartServices = true
			shouldSave = true
		}

		if shouldSave {
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
			ApachePort:        8080,
			NginxPort:         80,
			MySQLPort:         3306,
		}
	}

	return prefs
}

func (p *Preferences) Save() error {
	configDir := utils.GetStackerDir()
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
