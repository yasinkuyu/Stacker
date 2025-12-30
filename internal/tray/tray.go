package tray

import (
	_ "embed"
	"fmt"
	"os/exec"
	"runtime"

	"fyne.io/systray"
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
	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Stackr")
		systray.SetTooltip("Stackr - PHP Development Environment")

		mOpen := systray.AddMenuItem("Open Dashboard", "Open web dashboard")
		systray.AddSeparator()
		mStatus := systray.AddMenuItem("Status: Running", "Show status")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit Stackr")

		go func() {
			<-mOpen.ClickedCh
			tm.openBrowser()
		}()
		go func() {
			<-mStatus.ClickedCh
			tm.showStatus()
		}()
		go func() {
			<-mQuit.ClickedCh
			systray.Quit()
		}()
	}, func() {
		close(tm.quitChan)
	})
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
	fmt.Printf("\n✓ Stackr Status: Running\n")
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
