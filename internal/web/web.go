package web

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yasinkuyu/Stacker/internal/config"
	"github.com/yasinkuyu/Stacker/internal/dumps"
	"github.com/yasinkuyu/Stacker/internal/logs"
	"github.com/yasinkuyu/Stacker/internal/mail"
	"github.com/yasinkuyu/Stacker/internal/php"
	"github.com/yasinkuyu/Stacker/internal/services"
	"github.com/yasinkuyu/Stacker/internal/utils"
)

//go:embed index.html
var indexHTML string

//go:embed logo.png
var logoPNG []byte

//go:embed services/*.svg
var serviceLogos embed.FS

//go:embed locales/*.json
var localeFS embed.FS

// Site represents a local development site
type Site struct {
	Name string `json:"name"`
	Path string `json:"path"`
	PHP  string `json:"php,omitempty"`
	SSL  bool   `json:"ssl"`
}

// Preferences holds user settings
type Preferences struct {
	Theme     string `json:"theme"`
	AutoStart bool   `json:"autoStart"`
	ShowTray  bool   `json:"showTray"`
	Port      int    `json:"port"`
	SlimMode  bool   `json:"slimMode"`
}

var (
	prefs     = Preferences{Theme: "dark", AutoStart: false, ShowTray: true, Port: 9999, SlimMode: false}
	prefMutex sync.RWMutex
	sites     = make([]Site, 0)
	sitesMu   sync.RWMutex
)

type WebServer struct {
	config          *config.Config
	dumpManager     *dumps.DumpManager
	mailManager     *mail.MailManager
	serviceManager  *services.ServiceManager
	fpmManager      *php.FPMManager
	phpManager      *php.PHPManager
	stackerDir      string
	installProgress map[string]int
	progressMu      sync.RWMutex
}

func NewWebServer(cfg *config.Config) *WebServer {
	sm := services.NewServiceManager()
	stackerDir := utils.GetStackerDir()

	// Load saved sites
	loadSites(stackerDir)
	loadPreferences(stackerDir)

	// Initialize PHP managers
	pm := php.NewPHPManager()
	pm.DetectPHPVersions()
	fm := php.NewFPMManager()

	return &WebServer{
		config:          cfg,
		dumpManager:     dumps.NewDumpManager(cfg),
		mailManager:     mail.NewMailManager(cfg),
		serviceManager:  sm,
		fpmManager:      fm,
		phpManager:      pm,
		stackerDir:      stackerDir,
		installProgress: make(map[string]int),
	}
}

func loadSites(stackerDir string) {
	sitesFile := filepath.Join(stackerDir, "sites.json")
	data, err := os.ReadFile(sitesFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &sites)
}

func saveSites(stackerDir string) {
	sitesFile := filepath.Join(stackerDir, "sites.json")
	data, _ := json.MarshalIndent(sites, "", "  ")
	os.WriteFile(sitesFile, data, 0644)
}

func loadPreferences(stackerDir string) {
	prefsFile := filepath.Join(stackerDir, "preferences.json")
	data, err := os.ReadFile(prefsFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &prefs)
}

func savePreferences(stackerDir string) {
	prefsFile := filepath.Join(stackerDir, "preferences.json")
	data, _ := json.MarshalIndent(prefs, "", "  ")
	os.WriteFile(prefsFile, data, 0644)
}

func (ws *WebServer) Start() error {
	// API endpoints
	http.HandleFunc("/", ws.handleIndex)
	http.HandleFunc("/logo.png", ws.handleLogo)
	http.HandleFunc("/static/logo.png", ws.handleLogo)

	http.HandleFunc("/api/status", ws.handleStatus)
	http.HandleFunc("/api/sites", ws.handleSites)
	http.HandleFunc("/api/sites/", ws.handleSiteByName)
	http.HandleFunc("/api/services", ws.handleServices)
	http.HandleFunc("/api/services/versions", ws.handleServiceVersions)
	http.HandleFunc("/api/services/install", ws.handleServiceInstall)
	http.HandleFunc("/api/services/install-status", ws.handleServiceInstallStatus)
	http.HandleFunc("/api/services/progress/stream", ws.handleInstallProgressSSE)
	http.HandleFunc("/api/services/health/stream", ws.handleServiceHealthSSE)
	http.HandleFunc("/api/services/uninstall", ws.handleServiceUninstall)
	http.HandleFunc("/api/services/start/", ws.handleServiceStart)
	http.HandleFunc("/api/services/stop/", ws.handleServiceStop)
	http.HandleFunc("/api/services/restart/", ws.handleServiceRestart)
	http.HandleFunc("/api/services/config/", ws.handleServiceConfig)
	http.HandleFunc("/api/dumps", ws.handleDumps)
	http.HandleFunc("/api/mail", ws.handleMail)
	http.HandleFunc("/api/logs", ws.handleLogs)
	http.HandleFunc("/api/logs/view", ws.handleLogView)
	http.HandleFunc("/api/php", ws.handlePHP)
	http.HandleFunc("/api/php/install", ws.handlePHPInstall)
	http.HandleFunc("/api/php/install-status", ws.handlePHPInstallStatus)
	http.HandleFunc("/api/php/default", ws.handlePHPDefault)
	http.HandleFunc("/api/preferences", ws.handlePreferences)
	http.HandleFunc("/api/locales/", ws.handleLocales)
	http.HandleFunc("/api/open-folder", ws.handleOpenFolder)
	http.HandleFunc("/api/open-terminal", ws.handleOpenTerminal)
	http.HandleFunc("/api/browse-folder", ws.handleBrowseFolder)
	http.HandleFunc("/api/dumps/ingest", ws.handleDumpIngest)

	logoFS, _ := fs.Sub(serviceLogos, "services")
	http.Handle("/api/static/services/", http.StripPrefix("/api/static/services/", http.FileServer(http.FS(logoFS))))

	ws.mailManager.Start()

	// Auto-start PHP-FPM pools for configured sites
	ws.startRequiredFPMPools()

	// Start background service status worker (checks every 3 seconds)
	ws.serviceManager.StartStatusWorker(3 * time.Second)

	prefMutex.RLock()
	port := prefs.Port
	prefMutex.RUnlock()

	fmt.Printf("🚀 Web UI starting on http://localhost:%d\n", port)
	fmt.Printf("📁 Data directory: %s\n", ws.stackerDir)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
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

	runningCount := 0
	for _, svc := range ws.serviceManager.GetServices() {
		if svc.Status == "running" {
			runningCount++
		}
	}

	dumpCount := len(ws.dumpManager.GetDumps())

	sitesMu.RLock()
	siteCount := len(sites)
	sitesMu.RUnlock()

	status := map[string]interface{}{
		"status":     "running",
		"version":    "1.0.0",
		"sites":      siteCount,
		"services":   runningCount,
		"dumps":      dumpCount,
		"php":        phpVersion,
		"stackerDir": ws.stackerDir,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ===========================================
// SITES API - FULLY FUNCTIONAL
// ===========================================

func (ws *WebServer) handleSites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		sitesMu.RLock()
		defer sitesMu.RUnlock()
		json.NewEncoder(w).Encode(sites)

	case "POST":
		var site Site
		if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate
		if site.Name == "" || site.Path == "" {
			http.Error(w, "Name and path required", http.StatusBadRequest)
			return
		}

		// Create site directory if needed
		sitePath := filepath.Join(ws.stackerDir, "sites", site.Name)
		os.MkdirAll(sitePath, 0755)

		// Pin site to PHP version if specified
		if site.PHP != "" {
			ws.phpManager.PinSite(site.Name, site.PHP)
			// Start PHP-FPM pool for this version
			if err := ws.fpmManager.EnsureRunning(site.PHP); err != nil {
				fmt.Printf("⚠️ Failed to start PHP-FPM %s: %v\n", site.PHP, err)
			}
		}

		// Add to hosts file simulation - create a config file
		if err := ws.createSiteConfig(site); err != nil {
			http.Error(w, "Failed to create site config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Reload Nginx if it's running
		ws.serviceManager.RestartService("nginx-active") // We need a way to find the active nginx
		// Actually, let's just attempt to reload any service of type nginx
		for _, svc := range ws.serviceManager.GetServices() {
			if svc.Type == "nginx" && svc.Status == "running" {
				ws.serviceManager.RestartService(svc.Name)
			}
		}

		sitesMu.Lock()
		sites = append(sites, site)
		sitesMu.Unlock()
		saveSites(ws.stackerDir)

		json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": site.Name})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *WebServer) handleSiteByName(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract site name from URL: /api/sites/sitename
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Site name required", http.StatusBadRequest)
		return
	}
	siteName := parts[3]

	switch r.Method {
	case "PUT":
		var updatedSite Site
		if err := json.NewDecoder(r.Body).Decode(&updatedSite); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Handle PHP version change
		if updatedSite.PHP != "" {
			ws.phpManager.PinSite(updatedSite.Name, updatedSite.PHP)
			if err := ws.fpmManager.EnsureRunning(updatedSite.PHP); err != nil {
				fmt.Printf("⚠️ Failed to start PHP-FPM %s: %v\n", updatedSite.PHP, err)
			}
		} else {
			ws.phpManager.UnpinSite(updatedSite.Name)
		}

		sitesMu.Lock()
		for i, s := range sites {
			if s.Name == siteName {
				sites[i] = updatedSite
				break
			}
		}
		sitesMu.Unlock()
		saveSites(ws.stackerDir)

		ws.createSiteConfig(updatedSite)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	case "DELETE":
		sitesMu.Lock()
		for i, s := range sites {
			if s.Name == siteName {
				sites = append(sites[:i], sites[i+1:]...)
				break
			}
		}
		sitesMu.Unlock()
		saveSites(ws.stackerDir)

		// Remove site config
		configPath := filepath.Join(ws.stackerDir, "conf", "nginx", siteName+".conf")
		os.Remove(configPath)

		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *WebServer) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", req.Path)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", req.Path)
	} else {
		cmd = exec.Command("xdg-open", req.Path)
	}

	cmd.Run()
	w.WriteHeader(http.StatusOK)
}

func (ws *WebServer) handleOpenTerminal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-a", "Terminal", req.Path)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/K", "cd", "/d", req.Path)
	} else {
		cmd = exec.Command("x-terminal-emulator", "--working-directory", req.Path)
	}

	cmd.Run()
	w.WriteHeader(http.StatusOK)
}

func (ws *WebServer) handleBrowseFolder(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "darwin" {
		http.Error(w, "Only supported on macOS", http.StatusNotImplemented)
		return
	}

	cmd := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Select Project Folder")`)
	out, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimSpace(string(out))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": path})
}

func (ws *WebServer) createSiteConfig(site Site) error {
	confDir := filepath.Join(ws.stackerDir, "conf", "nginx")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(confDir, site.Name+".conf")

	phpPort := ws.getPHPPort(site.PHP)

	// Detect document root
	docRoot := site.Path
	if _, err := os.Stat(filepath.Join(site.Path, "public")); err == nil {
		docRoot = filepath.Join(site.Path, "public")
	}

	config := fmt.Sprintf(`# Stacker Site Config: %[1]s
# Generated: %[2]s
server {
    listen 80;
    server_name %[1]s.test;
    root "%[3]s";
    index index.php index.html index.htm;

    client_max_body_size 100M;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:%[4]d;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }

    location ~ /\.ht {
        deny all;
    }
}
`, site.Name, time.Now().Format(time.RFC3339), docRoot, phpPort)

	return os.WriteFile(configPath, []byte(config), 0644)
}

type ProgressReader struct {
	io.Reader
	Total   int64
	Current int64
	OnProg  func(int)
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 {
		pr.OnProg(int(float64(pr.Current) / float64(pr.Total) * 100))
	}
	return
}

func (ws *WebServer) downloadAndExtractPHP(version, targetDir string) error {
	fullVersion := config.GetFullVersion("php", version)

	// Get available PHP versions to find the correct URL
	phpVers := config.GetAvailableVersions("php", "")
	var urls []string
	for _, v := range phpVers {
		if v.Version == version {
			urls = []string{v.URL}
			break
		}
	}

	if len(urls) == 0 {
		return fmt.Errorf("PHP version %s not found in remote config", version)
	}

	var lastErr error
	for i, url := range urls {
		fmt.Printf("⬇️ Downloading PHP %s from %s...\n", fullVersion, url)

		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			fmt.Printf("❌ Failed (attempt %d): %v\n", i+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("failed to download PHP: %s (status %d)", url, resp.StatusCode)
			fmt.Printf("❌ Failed (attempt %d): HTTP %d\n", i+1, resp.StatusCode)
			continue
		}

		// Successfully got response, proceed with extraction
		defer resp.Body.Close()

		// Set progress to 0
		ws.progressMu.Lock()
		ws.installProgress[version] = 0
		ws.progressMu.Unlock()

		pr := &ProgressReader{
			Reader: resp.Body,
			Total:  resp.ContentLength,
			OnProg: func(p int) {
				ws.progressMu.Lock()
				ws.installProgress[version] = p
				ws.progressMu.Unlock()
			},
		}

		gzr, err := gzip.NewReader(pr)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tr := tar.NewReader(gzr)

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			target := filepath.Join(targetDir, header.Name)

			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			case tar.TypeReg:
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					return err
				}
				if _, err := io.Copy(f, tr); err != nil {
					f.Close()
					return err
				}
				f.Close()
			}
		}

		// Ensure binary is named 'php' and is executable
		phpBinary := filepath.Join(targetDir, "php")
		if _, err := os.Stat(phpBinary); os.IsNotExist(err) {
			// Try to find php in subdirectories
			entries, _ := os.ReadDir(targetDir)
			for _, e := range entries {
				if e.IsDir() {
					subPhp := filepath.Join(targetDir, e.Name(), "php")
					if _, err := os.Stat(subPhp); err == nil {
						phpBinary = subPhp
						break
					}
				}
			}
		}
		os.Chmod(phpBinary, 0755)

		// Set progress to 100
		ws.progressMu.Lock()
		ws.installProgress[version] = 100
		ws.progressMu.Unlock()

		fmt.Printf("✅ PHP %s installed to %s\n", version, phpBinary)
		return nil
	}

	// All URLs failed - provide installation guide
	ws.progressMu.Lock()
	ws.installProgress[version] = -1
	ws.progressMu.Unlock()

	// Print helpful installation commands
	fmt.Printf("\n❌ Auto-download failed. Please install PHP manually:\n\n")
	fmt.Printf("For macOS:\n")
	fmt.Printf("  brew install php%s\n", version)
	fmt.Printf("  brew install php@%s\n", version)
	fmt.Printf("\nFor Ubuntu/Debian:\n")
	fmt.Printf("  sudo apt install php%s\n", version)
	fmt.Printf("\nFor Windows:\n")
	fmt.Printf("  winget install PHP.php%s\n", version)
	fmt.Printf("  Or download from: https://windows.php.net/download/\n")
	fmt.Printf("\nAfter installation, Stacker will detect it automatically.\n")

	return lastErr
}

// GetPlatformPHPInstallCommand returns platform-specific install commands
func (ws *WebServer) GetPlatformPHPInstallCommands(version string) map[string]string {
	commands := make(map[string]string)
	osName := runtime.GOOS

	switch osName {
	case "darwin":
		commands["brew"] = fmt.Sprintf("brew install php%s", version)
		commands["brew_versioned"] = fmt.Sprintf("brew install php@%s", version)
	case "linux":
		commands["apt"] = fmt.Sprintf("sudo apt install php%s", version)
		commands["yum"] = fmt.Sprintf("sudo yum install php%s", version)
		commands["dnf"] = fmt.Sprintf("sudo dnf install php%s", version)
	case "windows":
		commands["winget"] = fmt.Sprintf("winget install PHP.php%s", version)
		commands["url"] = "https://windows.php.net/download/"
	default:
		commands["generic"] = fmt.Sprintf("Install PHP %s from https://php.net/downloads.php", version)
	}

	return commands
}

func (ws *WebServer) getPHPPort(version string) int {
	if version == "" {
		pm := php.NewPHPManager()
		pm.DetectPHPVersions()
		if def := pm.GetDefault(); def != nil {
			version = def.Version
		} else {
			return 9000 // Last fallback
		}
	}

	// Map versions like 8.3 -> 9083, 7.4 -> 9074
	// Remove dots and prefix with 90
	clean := strings.ReplaceAll(version, ".", "")
	var port int
	fmt.Sscanf(clean, "%d", &port)

	if port == 0 {
		return 9000
	}

	// If port is < 100 (like 83), prefix with 90
	if port < 100 {
		return 9000 + port
	}

	return port
}

// ===========================================
// SERVICES API - FULLY FUNCTIONAL
// ===========================================

func (ws *WebServer) handleServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	svcs := ws.serviceManager.GetServices()
	// Update status for each service before returning
	for _, svc := range svcs {
		ws.serviceManager.GetStatus(svc.Name)
	}

	available := ws.serviceManager.GetAvailableVersions("")

	// Build list of installed version keys
	installedVersions := make([]string, 0)
	for _, svc := range svcs {
		installedVersions = append(installedVersions, svc.Type+"-"+svc.Version)
	}

	response := map[string]interface{}{
		"installed":         svcs,
		"available":         available,
		"installedVersions": installedVersions,
	}

	json.NewEncoder(w).Encode(response)
}

func filterAvailable(available []services.ServiceVersion, installed []*services.Service) []services.ServiceVersion {
	var filtered []services.ServiceVersion
	installedMap := make(map[string]bool)
	for _, svc := range installed {
		installedMap[svc.Type+"-"+svc.Version] = true
	}

	for _, av := range available {
		if !installedMap[av.Type+"-"+av.Version] {
			filtered = append(filtered, av)
		}
	}
	return filtered
}

func (ws *WebServer) handleServiceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type    string `json:"type"`
		Version string `json:"version"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	go func() {
		if err := ws.serviceManager.InstallService(req.Type, req.Version); err != nil {
			// Set progress to -1 on error and store error message
			ws.serviceManager.SetInstallError(req.Type, req.Version, err.Error())
			fmt.Printf("Error installing service: %v\n", err)
		} else {
			// Auto-start after install (except for tools like composer/nodejs)
			if req.Type != "composer" && req.Type != "nodejs" {
				svcName := req.Type + "-" + req.Version
				fmt.Printf("🚀 Auto-starting %s after install...\n", svcName)
				if err := ws.serviceManager.StartService(svcName); err != nil {
					fmt.Printf("⚠️ Failed to auto-start %s: %v\n", svcName, err)
				}
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "installing", "type": req.Type, "version": req.Version})
}

func (ws *WebServer) handleServiceVersions(w http.ResponseWriter, r *http.Request) {
	svcType := r.URL.Query().Get("type")
	versions := ws.serviceManager.GetAvailableVersions(svcType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"versions": versions})
}

func (ws *WebServer) handleServiceInstallStatus(w http.ResponseWriter, r *http.Request) {
	svcType := r.URL.Query().Get("type")
	version := r.URL.Query().Get("version")

	if svcType == "" || version == "" {
		http.Error(w, "Type and version required", http.StatusBadRequest)
		return
	}

	progress, errMsg := ws.serviceManager.GetInstallStatus(svcType, version)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"progress": progress,
		"error":    errMsg,
	})
}

func (ws *WebServer) handleServiceUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := ws.serviceManager.UninstallService(req.Name); err != nil {
		utils.LogError(fmt.Sprintf("Failed to uninstall service %s: %v", req.Name, err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogService(req.Name, "uninstall", "success")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "uninstalled", "name": req.Name})
}

func (ws *WebServer) handleServiceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Service name required", http.StatusBadRequest)
		return
	}
	serviceName := parts[4]

	if err := ws.serviceManager.StartService(serviceName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "name": serviceName})
}

func (ws *WebServer) handleServiceStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Service name required", http.StatusBadRequest)
		return
	}
	serviceName := parts[4]

	if err := ws.serviceManager.StopService(serviceName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "name": serviceName})
}

func (ws *WebServer) handleServiceRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Service name required", http.StatusBadRequest)
		return
	}
	serviceName := parts[4]

	if err := ws.serviceManager.RestartService(serviceName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarted", "name": serviceName})
}

func (ws *WebServer) handleServiceConfig(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Service name required", http.StatusBadRequest)
		return
	}
	serviceName := parts[4]

	if r.Method == "GET" {
		configPath, content, err := ws.serviceManager.GetServiceConfig(serviceName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":    serviceName,
			"path":    configPath,
			"content": content,
		})
		return
	}

	if r.Method == "POST" {
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := ws.serviceManager.SaveServiceConfig(serviceName, req.Content); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "saved", "name": serviceName})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

func (ws *WebServer) handleDumpIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit payload to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Try to determine site from Referer or Host or Custom Header
	siteName := "Unknown"
	if referer := r.Header.Get("Referer"); referer != "" {
		siteName = referer
	} else if origin := r.Header.Get("Origin"); origin != "" {
		siteName = origin
	}

	if err := ws.dumpManager.HandleLaravelDumpRequest(body, siteName); err != nil {
		// Fallback to simple dump if structure doesn't match
		ws.dumpManager.ParseLaravelDump(string(body), siteName)
	}

	w.WriteHeader(http.StatusOK)
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

	// Add global logs
	baseDir := utils.GetStackerDir()
	lm.AddLogDir("Global", filepath.Join(baseDir, "logs"))

	// Add site logs (Laravel support)
	sites := ws.config.GetSites()
	for _, site := range sites {
		laravelLog := filepath.Join(site.Path, "storage", "logs")
		if _, err := os.Stat(laravelLog); err == nil {
			lm.AddLogDir(site.Name, laravelLog)
		}
	}

	logFiles := lm.GetLogFiles()

	w.Header().Set("Content-Type", "application/json")
	if logFiles == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(logFiles)
}

func (ws *WebServer) handleLogView(w http.ResponseWriter, r *http.Request) {
	logPath := r.URL.Query().Get("path")
	if logPath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	// Security check: ensure path is within stacker dir or site dirs
	// For now, simple read
	content, err := os.ReadFile(logPath)
	if err != nil {
		http.Error(w, "Failed to read log file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path":    logPath,
		"content": string(content),
	})
}

// ===========================================
// PHP API - FULLY FUNCTIONAL
// ===========================================

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

func (ws *WebServer) handlePHPInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version string `json:"version"`
		XDebug  bool   `json:"xdebug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Create PHP directory structure
	phpBinDir := filepath.Join(ws.stackerDir, "bin", "php"+req.Version, "bin")
	confDir := filepath.Join(ws.stackerDir, "conf", "php")
	os.MkdirAll(phpBinDir, 0755)
	os.MkdirAll(confDir, 0755)

	// Download real PHP binary in background
	go func(version string, xdebug bool, binDir string, cDir string) {
		err := ws.downloadAndExtractPHP(version, binDir)
		if err != nil {
			fmt.Printf("Error downloading PHP %s: %v\n", version, err)
			return
		}

		// Create a status file
		statusFile := filepath.Join(ws.stackerDir, "bin", "php"+version, "status.json")
		statusData := map[string]interface{}{
			"version":   version,
			"xdebug":    xdebug,
			"installed": time.Now().Format(time.RFC3339),
			"status":    "installed",
		}
		data, _ := json.MarshalIndent(statusData, "", "  ")
		os.WriteFile(statusFile, data, 0644)

		// Create php.ini
		phpIni := fmt.Sprintf(`; Stacker PHP %s Configuration
; Generated: %s

[PHP]
memory_limit = 512M
upload_max_filesize = 100M
post_max_size = 100M
max_execution_time = 300
display_errors = On
error_reporting = E_ALL

[Date]
date.timezone = UTC
`, version, time.Now().Format(time.RFC3339))

		if xdebug {
			phpIni += `
[xdebug]
zend_extension=xdebug
xdebug.mode=debug
xdebug.client_host=127.0.0.1
xdebug.client_port=9003
xdebug.start_with_request=trigger
`
		}

		os.WriteFile(filepath.Join(cDir, "php"+version+".ini"), []byte(phpIni), 0644)
		fmt.Printf("✅ PHP %s configuration finalized\n", version)
	}(req.Version, req.XDebug, phpBinDir, confDir)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "starting", "version": req.Version})
}

func (ws *WebServer) handlePHPInstallStatus(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "Missing version", http.StatusBadRequest)
		return
	}

	ws.progressMu.RLock()
	progress := ws.installProgress[version]
	ws.progressMu.RUnlock()

	json.NewEncoder(w).Encode(map[string]int{"progress": progress})
}

func (ws *WebServer) handlePHPDefault(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	pm := php.NewPHPManager()
	pm.DetectPHPVersions()
	if err := pm.SetDefault(req.Version); err != nil {
		utils.LogError(fmt.Sprintf("Failed to set default PHP to %s: %v", req.Version, err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogInfo(fmt.Sprintf("PHP default version changed to %s", req.Version))

	// Save default version
	defaultFile := filepath.Join(ws.stackerDir, "php_default.txt")
	os.WriteFile(defaultFile, []byte(req.Version), 0644)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "set", "version": req.Version})
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
		if slimMode, ok := updates["slimMode"].(bool); ok {
			prefs.SlimMode = slimMode
		}
		if port, ok := updates["port"].(float64); ok {
			prefs.Port = int(port)
		}

		savePreferences(ws.stackerDir)

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

// OpenFolder opens a folder in Finder/Explorer
func OpenFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// Unused but reserved for future
var _ = io.EOF

func (ws *WebServer) handleInstallProgressSSE(w http.ResponseWriter, r *http.Request) {
	svcType := r.URL.Query().Get("type")
	version := r.URL.Query().Get("version")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, _ := w.(http.Flusher)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			progress := ws.serviceManager.GetInstallProgress(svcType, version)
			if progress >= 100 || progress < 0 {
				fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]interface{}{
					"progress": progress,
					"phase":    "complete",
				}))
				flusher.Flush()
				return
			}

			fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]interface{}{
				"progress": progress,
				"phase":    "downloading",
			}))
			flusher.Flush()
		}
	}
}

func (ws *WebServer) handleServiceHealthSSE(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("name")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, _ := w.(http.Flusher)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			health := ws.serviceManager.GetDetailedStatus(serviceName)
			fmt.Fprintf(w, "data: %s\n\n", toJSON(health))
			flusher.Flush()
		}
	}
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// startRequiredFPMPools starts PHP-FPM pools for sites with pinned PHP versions
func (ws *WebServer) startRequiredFPMPools() {
	sitesMu.RLock()
	defer sitesMu.RUnlock()

	// Collect unique PHP versions used by sites
	versions := make(map[string]bool)
	for _, site := range sites {
		if site.PHP != "" {
			versions[site.PHP] = true
		}
	}

	// Start FPM pool for each unique version
	for version := range versions {
		if err := ws.fpmManager.EnsureRunning(version); err != nil {
			fmt.Printf("⚠️ Failed to start PHP-FPM %s: %v\n", version, err)
		}
	}
}
func (ws *WebServer) handleLocales(w http.ResponseWriter, r *http.Request) {
	// Extract language from URL: /api/locales/en
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Language required", http.StatusBadRequest)
		return
	}
	lang := parts[3]
	if lang == "" {
		lang = "en"
	}

	// Read from embedded FS
	data, err := localeFS.ReadFile("locales/" + lang + ".json")
	if err != nil {
		// Fallback to English
		data, err = localeFS.ReadFile("locales/en.json")
		if err != nil {
			http.Error(w, "Locale not found", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
