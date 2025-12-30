//go:build !no_tray
// +build !no_tray

package tray

import (
	_ "embed"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

//go:embed icon.png
var iconData []byte

type TrayManager struct {
	webURL   string
	quitChan chan bool
}

func NewTrayManager() *TrayManager {
	return &TrayManager{
		quitChan: make(chan bool),
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
	systray.SetTitle("Stacker")
	systray.SetTooltip("Stacker - PHP Development Environment")

	mOpen := systray.AddMenuItem("Open Dashboard", "Open web dashboard")
	systray.AddSeparator()
	mStatus := systray.AddMenuItem("Status: Running", "Show status")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Stacker")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				tm.openBrowser()
			case <-mStatus.ClickedCh:
				tm.showStatus()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func (tm *TrayManager) onExit() {
	close(tm.quitChan)
}

func (tm *TrayManager) openBrowser() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", tm.webURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", tm.webURL)
	default: // linux
		cmd = exec.Command("xdg-open", tm.webURL)
	}

	go cmd.Start()
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
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}

	cmd.Start()
}
