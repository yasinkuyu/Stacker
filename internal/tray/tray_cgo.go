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

	"github.com/getlantern/systray"
	"github.com/yasinkuyu/Stacker/internal/php"
	"github.com/yasinkuyu/Stacker/internal/services"
)

//go:embed icon.png
var iconData []byte

type TrayManager struct {
	webURL        string
	quitChan      chan bool
	phpManager    *php.PHPManager
	svcManager    *services.ServiceManager
	xdebugEnabled bool
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

	// Individual services
	mMySQL := mServices.AddSubMenuItem("MySQL", "MySQL Database")
	mRedis := mServices.AddSubMenuItem("Redis", "Redis Cache")
	mMeilisearch := mServices.AddSubMenuItem("Meilisearch", "Meilisearch")
	mMinIO := mServices.AddSubMenuItem("MinIO", "MinIO Object Storage")

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
