package xdebug

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type XDebugManager struct {
	enabled    bool
	autoDetect bool
	port       int
	ideKey     string
}

func NewXDebugManager() *XDebugManager {
	return &XDebugManager{
		enabled:    false,
		autoDetect: true,
		port:       9003,
		ideKey:     "PHPSTORM",
	}
}

func (xm *XDebugManager) IsEnabled() bool {
	return xm.enabled
}

func (xm *XDebugManager) SetEnabled(enabled bool) {
	xm.enabled = enabled
}

func (xm *XDebugManager) GetPort() int {
	return xm.port
}

func (xm *XDebugManager) SetPort(port int) {
	xm.port = port
}

func (xm *XDebugManager) GetIDEKey() string {
	return xm.ideKey
}

func (xm *XDebugManager) SetIDEKey(ideKey string) {
	xm.ideKey = ideKey
}

func (xm *XDebugManager) DetectBrowserExtension() bool {
	// Xdebug browser extension kontrolü
	// XDEBUG_SESSION cookie veya X-Debug-Token header kontrolü
	return false
}

func (xm *XDebugManager) GetXDebugConfig() string {
	return fmt.Sprintf(`
zend_extension=xdebug
xdebug.mode=debug
xdebug.start_with_request=yes
xdebug.client_host=localhost
xdebug.client_port=%d
xdebug.idekey=%s
`, xm.port, xm.ideKey)
}

func (xm *XDebugManager) GenerateXDebugIni() string {
	return xm.GetXDebugConfig()
}

func (xm *XDebugManager) InstallXDebug(phpBinary string) error {
	// pecl install xdebug
	cmd := exec.Command("pecl", "install", "xdebug")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install XDebug: %w", err)
	}

	// Find php.ini location
	cmd = exec.Command(phpBinary, "--ini")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not find php.ini: %w", err)
	}

	re := regexp.MustCompile(`Loaded Configuration File:\s+(.+)`)
	matches := re.FindStringSubmatch(string(output))

	if len(matches) < 2 {
		return fmt.Errorf("could not parse php.ini location")
	}

	phpIni := strings.TrimSpace(matches[1])
	if phpIni == "(none)" {
		home, _ := os.UserHomeDir()
		phpIni = filepath.Join(home, ".stacker-app", "php", "php.ini")
	}

	// Add xdebug configuration
	xdebugConfig := xm.GenerateXDebugIni()

	f, err := os.OpenFile(phpIni, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("\n" + xdebugConfig + "\n")

	return nil
}

func (xm *XDebugManager) IsXDebugLoaded(phpBinary string) bool {
	cmd := exec.Command(phpBinary, "-m")
	output, _ := cmd.CombinedOutput()
	return strings.Contains(string(output), "xdebug")
}

func (xm *XDebugManager) GetXDebugVersion(phpBinary string) (string, error) {
	cmd := exec.Command(phpBinary, "-r", "echo phpversion('xdebug');")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
