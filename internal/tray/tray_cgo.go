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

	// Set service manager status change callback
	tm.svcManager.OnStatusChange = func() {
		tm.updateServiceStatus()
		tm.updateIconByStatus()
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

	// Default services (not installed)
	if len(svcs) == 0 {
		defaultServices := []string{"mysql", "mariadb", "nginx", "apache", "redis"}
		for _, svcType := range defaultServices {
			item := mServices.AddSubMenuItem(fmt.Sprintf("○ %s (not installed)", strings.ToUpper(svcType)), fmt.Sprintf("%s not installed", svcType))
			item.Disable()
			tm.serviceMenuItems[svcType] = item
		}
	}

	// Installed services
	for _, svc := range svcs {
		statusIcon := "○"
		if svc.Status == "running" {
			statusIcon = "●"
		}

		title := fmt.Sprintf("%s %s (%s)", statusIcon, strings.ToUpper(svc.Type), svc.Version)
		item := mServices.AddSubMenuItem(title, fmt.Sprintf("%s %s - %s", svc.Status, svc.Type, svc.Version))
		tm.serviceMenuItems[svc.Name] = item
		tm.serviceTitles[svc.Name] = title
	}

	systray.AddSeparator()

	// Tools
	mDumps := systray.AddMenuItem("Dumps", "View dumps")
	mMail := systray.AddMenuItem("Mail", "View captured emails")
	mLogs := systray.AddMenuItem("Logs", "View logs")

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

			case <-mDumps.ClickedCh:
				tm.openBrowserPath("/#dumps")

			case <-mMail.ClickedCh:
				tm.openBrowserPath("/#mail")

			case <-mLogs.ClickedCh:
				tm.openBrowserPath("/#logs")

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
		statusIcon := "○"
		for _, svc := range svcs {
			if svc.Name == name && svc.Status == "running" {
				statusIcon = "●"
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
	running := 0
	total := len(svcs)

	if total == 0 {
		systray.SetIcon(iconData)
		return
	}

	for _, s := range svcs {
		if s.Status == "running" {
			running++
		}
	}

	if running == total {
		systray.SetIcon(iconData)
	} else if running > 0 {
		systray.SetIcon(iconOrange)
	} else {
		systray.SetIcon(iconRed)
	}
}

func (tm *TrayManager) onExit() {
	fmt.Println("🛑 Stacker is shutting down...")

	// Request all services to stop
	tm.svcManager.Stop()

	timeout := time.After(tm.shutdownTimeout)
	done := make(chan struct{})

	go func() {
		if err := tm.svcManager.GracefulStopAll(); err != nil {
			fmt.Printf("⚠️ Graceful stop had errors: %v\n", err)
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		fmt.Println("✅ All services stopped gracefully")
	case <-timeout:
		fmt.Println("⚠️ Timeout, force stopping remaining services...")
		tm.svcManager.ForceStopAll()
	}

	tm.svcManager.Wait()
	close(tm.quitChan)
	fmt.Println("👋 Goodbye!")
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
