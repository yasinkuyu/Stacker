//go:build no_tray
// +build no_tray

package tray

import (
	"fmt"
	"os/exec"
	"runtime"
)

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
	// No tray support without CGO
	fmt.Println("ℹ️  System tray disabled (build without CGO)")
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
