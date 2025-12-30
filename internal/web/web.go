package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/yasinkuyu/Stacker/internal/config"
	"github.com/yasinkuyu/Stacker/internal/dumps"
	"github.com/yasinkuyu/Stacker/internal/logs"
	"github.com/yasinkuyu/Stacker/internal/mail"
	"github.com/yasinkuyu/Stacker/internal/php"
	"github.com/yasinkuyu/Stacker/internal/services"
)

//go:embed index.html
var indexHTML string

//go:embed logo.png
var logoPNG []byte

// Preferences holds user settings
type Preferences struct {
	Theme     string `json:"theme"`
	AutoStart bool   `json:"autoStart"`
	ShowTray  bool   `json:"showTray"`
	Port      int    `json:"port"`
}

var (
	prefs     = Preferences{Theme: "dark", AutoStart: false, ShowTray: true, Port: 8080}
	prefMutex sync.RWMutex
)

type WebServer struct {
	config      *config.Config
	dumpManager *dumps.DumpManager
	mailManager *mail.MailManager
}

func NewWebServer(cfg *config.Config) *WebServer {
	return &WebServer{
		config:      cfg,
		dumpManager: dumps.NewDumpManager(cfg),
		mailManager: mail.NewMailManager(cfg),
	}
}

func (ws *WebServer) Start() error {
	// Serve static files
	http.HandleFunc("/", ws.handleIndex)
	http.HandleFunc("/logo.png", ws.handleLogo)
	http.HandleFunc("/static/logo.png", ws.handleLogo)

	// API endpoints
	http.HandleFunc("/api/status", ws.handleStatus)
	http.HandleFunc("/api/sites", ws.handleSites)
	http.HandleFunc("/api/services", ws.handleServices)
	http.HandleFunc("/api/dumps", ws.handleDumps)
	http.HandleFunc("/api/mail", ws.handleMail)
	http.HandleFunc("/api/logs", ws.handleLogs)
	http.HandleFunc("/api/php", ws.handlePHP)
	http.HandleFunc("/api/preferences", ws.handlePreferences)

	fmt.Println("🚀 Web UI starting on http://localhost:8080")
	return http.ListenAndServe(":8080", nil)
}

func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func (ws *WebServer) handleLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(logoPNG)
}

func (ws *WebServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	pm := php.NewPHPManager()
	pm.DetectPHPVersions()
	defaultPHP := pm.GetDefault()
	phpVersion := "-"
	if defaultPHP != nil {
		phpVersion = defaultPHP.Version
	}

	sm := services.NewServiceManager()
	runningCount := 0
	for _, svc := range sm.GetServices() {
		if svc.Status == "running" {
			runningCount++
		}
	}

	dumpCount := len(ws.dumpManager.GetDumps())

	status := map[string]interface{}{
		"status":   "running",
		"version":  "1.0.0",
		"sites":    len(ws.config.Sites),
		"services": runningCount,
		"dumps":    dumpCount,
		"php":      phpVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (ws *WebServer) handleSites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if ws.config.Sites == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(ws.config.Sites)
}

func (ws *WebServer) handleServices(w http.ResponseWriter, r *http.Request) {
	sm := services.NewServiceManager()
	svcs := sm.GetServices()

	w.Header().Set("Content-Type", "application/json")
	if svcs == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(svcs)
}

func (ws *WebServer) handleDumps(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		ws.dumpManager.ClearDumps()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
		return
	}

	allDumps := ws.dumpManager.GetDumps()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"dumps": allDumps})
}

func (ws *WebServer) handleMail(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		ws.mailManager.ClearEmails()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
		return
	}

	emails := ws.mailManager.LoadEmails()
	w.Header().Set("Content-Type", "application/json")
	if emails == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(emails)
}

func (ws *WebServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	lm := logs.NewLogManager()
	logFiles := lm.GetLogFiles()

	w.Header().Set("Content-Type", "application/json")
	if logFiles == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(logFiles)
}

func (ws *WebServer) handlePHP(w http.ResponseWriter, r *http.Request) {
	pm := php.NewPHPManager()
	pm.DetectPHPVersions()

	type PHPVersion struct {
		Version string `json:"version"`
		Path    string `json:"path"`
		Default bool   `json:"default"`
	}

	var versions []PHPVersion
	defaultPHP := pm.GetDefault()

	for _, v := range pm.GetVersions() {
		versions = append(versions, PHPVersion{
			Version: v.Version,
			Path:    v.Path,
			Default: defaultPHP != nil && v.Version == defaultPHP.Version,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"versions": versions})
}

func (ws *WebServer) handlePreferences(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		prefMutex.RLock()
		defer prefMutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prefs)

	case "PUT":
		prefMutex.Lock()
		defer prefMutex.Unlock()

		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if theme, ok := updates["theme"].(string); ok {
			prefs.Theme = theme
		}
		if autoStart, ok := updates["autoStart"].(bool); ok {
			prefs.AutoStart = autoStart
			ws.updateAutoStart(autoStart)
		}
		if showTray, ok := updates["showTray"].(bool); ok {
			prefs.ShowTray = showTray
		}
		if port, ok := updates["port"].(float64); ok {
			prefs.Port = int(port)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prefs)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *WebServer) updateAutoStart(enable bool) {
	homeDir, _ := os.UserHomeDir()
	launchAgentsDir := filepath.Join(homeDir, "Library/LaunchAgents")
	plistPath := filepath.Join(launchAgentsDir, "com.insya.stacker.launcher.plist")

	if enable {
		os.MkdirAll(launchAgentsDir, 0755)
		exePath, _ := os.Executable()
		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.insya.stacker.launcher</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>tray</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>`, exePath)
		os.WriteFile(plistPath, []byte(plistContent), 0644)
	} else {
		os.Remove(plistPath)
	}
}
