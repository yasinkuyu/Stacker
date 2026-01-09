//go:build !no_tray
// +build !no_tray

package tray

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/yasinkuyu/Stacker/internal/config"
	"github.com/yasinkuyu/Stacker/internal/php"
	"github.com/yasinkuyu/Stacker/internal/services"
)

//go:embed icon.png
var iconData []byte

//go:embed icon_green.png
var iconGreen []byte

//go:embed icon_orange.png
var iconOrange []byte

//go:embed icon_red.png
var iconRed []byte

type TrayManager struct {
	webURL           string
	quitChan         chan bool
	phpManager       *php.PHPManager
	svcManager       *services.ServiceManager
	xdebugEnabled    bool
	serviceMenuItems map[string]*systray.MenuItem
	serviceTitles    map[string]string
	shutdownTimeout  time.Duration
}

func NewTrayManager() *TrayManager {
	return &TrayManager{
		quitChan:         make(chan bool),
		phpManager:       php.NewPHPManager(),
		svcManager:       services.NewServiceManager(),
		serviceMenuItems: make(map[string]*systray.MenuItem),
		serviceTitles:    make(map[string]string),
		shutdownTimeout:  10 * time.Second,
	}
}

func (tm *TrayManager) SetWebURL(url string) {
	tm.webURL = url
}

func (tm *TrayManager) Run() {
	systray.Run(tm.onReady, tm.onExit)
}

func (tm *TrayManager) onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("")
	systray.SetTooltip("Stacker - PHP Development Environment")

	tm.svcManager.OnStatusChange = func() {
		tm.updateServiceStatus()
		tm.updateIconByStatus()
	}

	// Auto-start services if enabled
	prefs := config.GetPreferences()
	if prefs.AutoStartServices {
		go func() {
			// Small delay to ensure everything is initialized
			time.Sleep(1 * time.Second)
			fmt.Println("🚀 Background worker: Auto-starting services...")
			tm.svcManager.StartAll()
		}()
	}

	// Open Dashboard
	mOpen := systray.AddMenuItem("Open Stacker", "Open web dashboard")

	systray.AddSeparator()

	// Sites submenu
	mSites := systray.AddMenuItem("Sites", "Manage sites")
	mAddSite := mSites.AddSubMenuItem("Add Site...", "Add a new site")
	mOpenSites := mSites.AddSubMenuItem("Open Sites Folder", "Open sites directory")

	systray.AddSeparator()

	// PHP Version submenu
	tm.phpManager.DetectPHPVersions()
	mPHP := systray.AddMenuItem("PHP", "Manage PHP versions")
	phpVersions := tm.phpManager.GetVersions()
	phpMenuItems := make([]*systray.MenuItem, 0)

	for _, v := range phpVersions {
		item := mPHP.AddSubMenuItem("PHP "+v.Version, "Open PHP "+v.Version+" directory")
		if v.Default {
			item.Check()
		}
		phpMenuItems = append(phpMenuItems, item)
	}

	// Services submenu

	// Services submenu
	mServices := systray.AddMenuItem("Services", "Manage services")
	mStartAll := mServices.AddSubMenuItem("Start All", "Start all services")
	mStopAll := mServices.AddSubMenuItem("Stop All", "Stop all services")

	// Get installed services and create menu items
	svcs := tm.svcManager.GetServices()

	// Default services (not installed)
	if len(svcs) == 0 {
		defaultServices := []string{"mysql", "mariadb", "nginx", "apache", "redis"}
		for _, svcType := range defaultServices {
			item := mServices.AddSubMenuItem(fmt.Sprintf("🔴 %s (not installed)", strings.ToUpper(svcType)), fmt.Sprintf("%s not installed", svcType))
			item.Disable()
			tm.serviceMenuItems[svcType] = item
		}
	}

	// Installed services
	for _, svc := range svcs {
		statusIcon := "🔴"
		if svc.Status == "running" {
			statusIcon = "🟢"
		}

		title := fmt.Sprintf("%s %s (%s)", statusIcon, strings.ToUpper(svc.Type), svc.Version)
		item := mServices.AddSubMenuItem(title, fmt.Sprintf("%s %s - %s", svc.Status, svc.Type, svc.Version))
		tm.serviceMenuItems[svc.Name] = item
		tm.serviceTitles[svc.Name] = title
	}

	systray.AddSeparator()

	// Settings
	mSettings := systray.AddMenuItem("Settings...", "Open settings")

	systray.AddSeparator()

	// Quit
	mQuit := systray.AddMenuItem("Quit Stacker", "Quit application")

	// Event handlers
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				tm.openBrowser()

			case <-mAddSite.ClickedCh:
				tm.openBrowserPath("/sites")

			case <-mOpenSites.ClickedCh:
				tm.openSitesFolder()

			case <-mStartAll.ClickedCh:
				svcs := tm.svcManager.GetServices()
				for _, svc := range svcs {
					if svc.Status != "running" {
						go tm.svcManager.StartService(svc.Name)
					}
				}

			case <-mStopAll.ClickedCh:
				svcs := tm.svcManager.GetServices()
				for _, svc := range svcs {
					if svc.Status == "running" {
						go tm.svcManager.StopService(svc.Name)
					}
				}

			case <-mSettings.ClickedCh:
				tm.openBrowserPath("/#settings")

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	// Handle PHP version clicks
	go func() {
		for i, item := range phpMenuItems {
			idx := i
			menuItem := item
			go func() {
				for range menuItem.ClickedCh {
					version := phpVersions[idx]
					tm.phpManager.SetDefault(version.Version)

					// Open PHP folder as requested
					phpDir := filepath.Dir(version.Path)
					// Usually PHP binaries are in bin/, so go one level up
					if strings.HasSuffix(phpDir, "bin") {
						phpDir = filepath.Dir(phpDir)
					}
					tm.openPath(phpDir)

					// Update checkmarks
					for j, mi := range phpMenuItems {
						if j == idx {
							mi.Check()
						} else {
							mi.Uncheck()
						}
					}
				}
			}()
		}
	}()

	// Handle service clicks
	go func() {
		for name, item := range tm.serviceMenuItems {
			serviceName := name
			menuItem := item
			go func() {
				for range menuItem.ClickedCh {
					svc := tm.svcManager.GetService(serviceName)
					if svc == nil {
						continue
					}

					if svc.Status == "running" {
						tm.svcManager.StopService(serviceName)
						menuItem.Uncheck()
					} else {
						tm.svcManager.StartService(serviceName)
						menuItem.Check()
					}

					tm.updateServiceStatus()
				}
			}()
		}
	}()

	// Watcher for service status to update tray icon
	go tm.watchStatus()
}

func (tm *TrayManager) watchStatus() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.updateServiceStatus()
			tm.updateIconByStatus()
		case <-tm.quitChan:
			return
		}
	}
}

func (tm *TrayManager) updateServiceStatus() {
	svcs := tm.svcManager.GetServices()

	for name, item := range tm.serviceMenuItems {
		statusIcon := "🔴"
		for _, svc := range svcs {
			if svc.Name == name && svc.Status == "running" {
				statusIcon = "🟢"
				break
			}
		}

		currentTitle := tm.serviceTitles[name]
		parts := strings.Fields(currentTitle)
		if len(parts) >= 2 {
			newTitle := fmt.Sprintf("%s %s", statusIcon, strings.Join(parts[1:], " "))
			item.SetTitle(newTitle)
			tm.serviceTitles[name] = newTitle
		}
	}
}

func (tm *TrayManager) updateIconByStatus() {
	svcs := tm.svcManager.GetServices()

	// Check only MySQL/MariaDB + (Apache or Nginx) for tray icon color
	mysqlRunning := false
	webServerRunning := false

	for _, s := range svcs {
		if s.Status != "running" {
			continue
		}
		// Check for database (MySQL or MariaDB)
		if s.Type == "mysql" || s.Type == "mariadb" {
			mysqlRunning = true
		}
		// Check for web server (Apache or Nginx)
		if s.Type == "apache" || s.Type == "nginx" {
			webServerRunning = true
		}
	}

	// Determine icon color based on critical services
	if mysqlRunning && webServerRunning {
		// All critical services running - normal/green icon
		systray.SetIcon(iconData)
	} else if mysqlRunning || webServerRunning {
		// Partial - orange icon
		systray.SetIcon(iconOrange)
	} else {
		// None running - red icon
		systray.SetIcon(iconRed)
	}
}

func (tm *TrayManager) onExit() {
	fmt.Println("🛑 Tray UI is closing...")
	// Signal internal goroutines to stop
	close(tm.quitChan)
	fmt.Println("👋 UI Closed")
}

// GetServiceManager returns the service manager for cleanup
func (tm *TrayManager) GetServiceManager() *services.ServiceManager {
	return tm.svcManager
}

func (tm *TrayManager) openBrowser() {
	tm.openURL(tm.webURL)
}

func (tm *TrayManager) openBrowserPath(path string) {
	tm.openURL(tm.webURL + path)
}

func (tm *TrayManager) openURL(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	go cmd.Start()
}

func (tm *TrayManager) openSitesFolder() {
	homeDir, _ := os.UserHomeDir()
	sitesPath := filepath.Join(homeDir, "Sites")
	os.MkdirAll(sitesPath, 0755)
	tm.openPath(sitesPath)
}

func (tm *TrayManager) openPath(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Start()
}

func (tm *TrayManager) showStatus() {
	fmt.Printf("\n✓ Stacker Status: Running\n")
	fmt.Printf("  Web UI: %s\n", tm.webURL)
}

func OpenBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	cmd.Start()
}
