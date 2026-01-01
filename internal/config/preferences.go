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

		// Auto-migrate from 8080 to 9999
		if prefs.Port == 8080 {
			prefs.Port = 9999
			// We can't easily call Save() here without being an instance method or duplicating logic,
			// but we can at least return the updated in-memory value directly.
			// Ideally we should save it, but let's just create a helper or just modify the struct.
			// Let's rely on the user saving later or just accept in-memory fix for now.
			// Actually, let's write it back so it persists.
			go func() {
				// Simple fire-and-forget save to update file
				p := &Preferences{
					Theme:     prefs.Theme,
					AutoStart: prefs.AutoStart,
					Port:      9999,
					ShowTray:  prefs.ShowTray,
				}
				data, _ := json.MarshalIndent(p, "", "  ")
				os.WriteFile(prefsPath, data, 0644)
			}()
		}
	} else {
		prefs = &Preferences{
			Theme:     "dark",
			AutoStart: false,
			Port:      9999,
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
