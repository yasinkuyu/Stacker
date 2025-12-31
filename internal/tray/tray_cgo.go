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

	"time"

	"github.com/getlantern/systray"
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
}

func NewTrayManager() *TrayManager {
	return &TrayManager{
		quitChan:   make(chan bool),
		phpManager: php.NewPHPManager(),
		svcManager: services.NewServiceManager(),
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
		item := mPHP.AddSubMenuItem("PHP "+v.Version, "Switch to PHP "+v.Version)
		if v.Default {
			item.Check()
		}
		phpMenuItems = append(phpMenuItems, item)
	}

	mPHP.AddSubMenuItem("", "")
	mXDebug := mPHP.AddSubMenuItem("XDebug: Off", "Toggle XDebug")

	systray.AddSeparator()

	// Services submenu
	mServices := systray.AddMenuItem("Services", "Manage services")
	mStartAll := mServices.AddSubMenuItem("Start All", "Start all services")
	mStopAll := mServices.AddSubMenuItem("Stop All", "Stop all services")
	mServices.AddSubMenuItem("", "")

	// Get installed services and create menu items
	svcs := tm.svcManager.GetServices()
	serviceMenuItems := make(map[string]*systray.MenuItem)

	// Default services (not installed)
	if len(svcs) == 0 {
		defaultServices := []string{"mysql", "mariadb", "nginx", "apache", "redis"}
		for _, svcType := range defaultServices {
			item := mServices.AddSubMenuItem(fmt.Sprintf("○ %s (not installed)", strings.ToUpper(svcType)), fmt.Sprintf("%s not installed", svcType))
			item.Disable()
			serviceMenuItems[svcType] = item
		}
	}

	// Installed services
	for _, svc := range svcs {
		statusIcon := "○"
		if svc.Status == "running" {
			statusIcon = "●"
		}

		item := mServices.AddSubMenuItem(
			fmt.Sprintf("%s %s (%s)", statusIcon, strings.ToUpper(svc.Type), svc.Version),
			fmt.Sprintf("%s %s - %s", svc.Status, svc.Type, svc.Version),
		)
		serviceMenuItems[svc.Name] = item
	}

	// Store for updates
	tm.serviceMenuItems = serviceMenuItems

	systray.AddSeparator()

	// Tools
	mDumps := systray.AddMenuItem("Dumps", "View dumps")
	mMail := systray.AddMenuItem("Mail", "View captured emails")
	mLogs := systray.AddMenuItem("Logs", "View logs")
	mLogsDrawer := mLogs.AddSubMenuItem("View Log Drawer", "Open log drawer in web UI")

	systray.AddSeparator()

	// Node.js submenu
	mNode := systray.AddMenuItem("Node.js", "Manage Node.js versions")
	mNodeCurrent := mNode.AddSubMenuItem("Current: -", "Current Node.js version")
	mNodeCurrent.Disable()

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

			case <-mXDebug.ClickedCh:
				tm.xdebugEnabled = !tm.xdebugEnabled
				if tm.xdebugEnabled {
					mXDebug.SetTitle("XDebug: On")
					mXDebug.Check()
				} else {
					mXDebug.SetTitle("XDebug: Off")
					mXDebug.Uncheck()
				}

			case <-mStartAll.ClickedCh:
				tm.svcManager.StartAll()

			case <-mStopAll.ClickedCh:
				tm.svcManager.StopAll()

			case <-mMySQL.ClickedCh:
				tm.toggleService("mysql", mMySQL)

			case <-mRedis.ClickedCh:
				tm.toggleService("redis", mRedis)

			case <-mMeilisearch.ClickedCh:
				tm.toggleService("meilisearch", mMeilisearch)

			case <-mMinIO.ClickedCh:
				tm.toggleService("minio", mMinIO)

			case <-mDumps.ClickedCh:
				tm.openBrowserPath("/#dumps")

			case <-mMail.ClickedCh:
				tm.openBrowserPath("/#mail")

			case <-mLogs.ClickedCh:
				tm.openBrowserPath("/#logs")

			case <-mLogsDrawer.ClickedCh:
				tm.openBrowserPath("/?drawer=logs")

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
					version := phpVersions[idx].Version
					tm.phpManager.SetDefault(version)
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

	// Watcher for service status to update tray icon
	go tm.watchStatus()
}

func (tm *TrayManager) watchStatus() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.updateIconByStatus()
		case <-tm.quitChan:
			return
		}
	}
}

func (tm *TrayManager) updateIconByStatus() {
	svcs := tm.svcManager.GetServices()
	running := 0
	total := len(svcs)

	if total == 0 {
		// If no services configured, show red or neutral
		systray.SetIcon(iconRed)
		return
	}

	for _, s := range svcs {
		if s.Status == "running" {
			running++
		}
	}

	if running == total && total > 0 {
		systray.SetIcon(iconGreen)
	} else if running > 0 {
		systray.SetIcon(iconOrange)
	} else {
		systray.SetIcon(iconRed)
	}
}

func (tm *TrayManager) onExit() {
	close(tm.quitChan)
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

	// Create if doesn't exist
	os.MkdirAll(sitesPath, 0755)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", sitesPath)
	case "windows":
		cmd = exec.Command("explorer", sitesPath)
	default:
		cmd = exec.Command("xdg-open", sitesPath)
	}
	cmd.Start()
}

func (tm *TrayManager) toggleService(name string, menuItem *systray.MenuItem) {
	svc := tm.svcManager.GetService(name)
	if svc == nil {
		return
	}

	if svc.Status == "running" {
		tm.svcManager.StopService(name)
		menuItem.Uncheck()
	} else {
		tm.svcManager.StartService(name)
		menuItem.Check()
	}
}

func (tm *TrayManager) updateServiceStatus() {
	svcs := tm.svcManager.GetServices()

	for name, item := range tm.serviceMenuItems {
		statusIcon := "○"
		for _, svc := range svcs {
			if svc.Name == name && svc.Status == "running" {
				statusIcon = "●"
				break
			}
		}

		parts := strings.Fields(item.Title())
		if len(parts) >= 2 {
			newTitle := fmt.Sprintf("%s %s", statusIcon, strings.Join(parts[1:], " "))
			item.SetTitle(newTitle)
		}
	}

	tm.updateIconByStatus()
}

func (tm *TrayManager) watchStatus() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.updateServiceStatus()
		case <-tm.quitChan:
			return
		}
	}
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
