package php

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/yasinkuyu/Stacker/internal/utils"
)

// FPMManager manages PHP-FPM pools for multiple PHP versions
type FPMManager struct {
	pools   map[string]*FPMPool // version -> pool
	mu      sync.RWMutex
	baseDir string
	confDir string
	logDir  string
	pidDir  string
}

// FPMPool represents a running PHP-FPM pool
type FPMPool struct {
	Version    string
	Port       int
	PID        int
	SocketPath string
	ConfigPath string
	Running    bool
}

// NewFPMManager creates a new FPM manager
func NewFPMManager() *FPMManager {
	baseDir := utils.GetStackerDir()

	fm := &FPMManager{
		pools:   make(map[string]*FPMPool),
		baseDir: baseDir,
		confDir: filepath.Join(baseDir, "conf", "php-fpm"),
		logDir:  filepath.Join(baseDir, "logs"),
		pidDir:  filepath.Join(baseDir, "pids"),
	}

	// Ensure directories exist
	os.MkdirAll(fm.confDir, 0755)
	os.MkdirAll(fm.logDir, 0755)
	os.MkdirAll(fm.pidDir, 0755)

	return fm
}

// GetPort returns the FPM port for a PHP version
// e.g., 7.4 -> 9074, 8.3 -> 9083
func GetPort(version string) int {
	clean := strings.ReplaceAll(version, ".", "")
	port, err := strconv.Atoi(clean)
	if err != nil || port == 0 {
		return 9000
	}
	if port < 100 {
		return 9000 + port
	}
	return port
}

// StartFPM starts a PHP-FPM pool for the given version
func (fm *FPMManager) StartFPM(version string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Check if already running
	if pool, exists := fm.pools[version]; exists && pool.Running {
		return nil // Already running
	}

	// Find PHP-FPM binary
	binary := fm.findFPMBinary(version)
	if binary == "" {
		return fmt.Errorf("PHP-FPM binary not found for version %s", version)
	}

	port := GetPort(version)

	// Generate config
	configPath, err := fm.generateConfig(version, port)
	if err != nil {
		return fmt.Errorf("failed to generate FPM config: %w", err)
	}

	// Start PHP-FPM
	cmd := exec.Command(binary, "-y", configPath, "-F")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start PHP-FPM: %w", err)
	}

	pool := &FPMPool{
		Version:    version,
		Port:       port,
		PID:        cmd.Process.Pid,
		ConfigPath: configPath,
		Running:    true,
	}

	fm.pools[version] = pool
	fm.savePID(version, cmd.Process.Pid)

	// Monitor process
	go fm.monitorProcess(version, cmd)

	fmt.Printf("✅ PHP-FPM %s started on port %d (PID: %d)\n", version, port, cmd.Process.Pid)
	return nil
}

// StopFPM stops a PHP-FPM pool
func (fm *FPMManager) StopFPM(version string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	pool, exists := fm.pools[version]
	if !exists {
		// Try to load PID from file
		pid := fm.loadPID(version)
		if pid > 0 {
			fm.killProcess(pid)
			fm.removePID(version)
		}
		return nil
	}

	if pool.PID > 0 {
		fm.killProcess(pool.PID)
	}

	pool.Running = false
	pool.PID = 0
	fm.removePID(version)

	fmt.Printf("⏹️ PHP-FPM %s stopped\n", version)
	return nil
}

// GetRunningFPM returns a list of running FPM versions
func (fm *FPMManager) GetRunningFPM() []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var running []string
	for version, pool := range fm.pools {
		if pool.Running {
			running = append(running, version)
		}
	}
	return running
}

// GetPool returns pool info for a version
func (fm *FPMManager) GetPool(version string) *FPMPool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.pools[version]
}

// IsRunning checks if FPM is running for a version
func (fm *FPMManager) IsRunning(version string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	pool, exists := fm.pools[version]
	if !exists {
		return false
	}
	return pool.Running
}

// findFPMBinary locates the php-fpm binary for a version
func (fm *FPMManager) findFPMBinary(version string) string {
	// Check Stacker installed PHP
	stackerPHP := filepath.Join(fm.baseDir, "bin", "php", version, "sbin", "php-fpm")
	if _, err := os.Stat(stackerPHP); err == nil {
		return stackerPHP
	}

	// Check Homebrew paths
	homebrewPaths := []string{
		fmt.Sprintf("/opt/homebrew/opt/php@%s/sbin/php-fpm", version),
		fmt.Sprintf("/usr/local/opt/php@%s/sbin/php-fpm", version),
		"/opt/homebrew/sbin/php-fpm",
		"/usr/local/sbin/php-fpm",
	}

	for _, path := range homebrewPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try generic php-fpm in PATH
	if path, err := exec.LookPath("php-fpm"); err == nil {
		return path
	}

	return ""
}

// generateConfig creates a PHP-FPM config file for the version
func (fm *FPMManager) generateConfig(version string, port int) (string, error) {
	configPath := filepath.Join(fm.confDir, fmt.Sprintf("php-fpm-%s.conf", version))
	pidFile := filepath.Join(fm.pidDir, fmt.Sprintf("php-fpm-%s.pid", version))
	errorLog := filepath.Join(fm.logDir, fmt.Sprintf("php-fpm-%s-error.log", version))

	config := fmt.Sprintf(`[global]
pid = %s
error_log = %s
daemonize = no

[www]
listen = 127.0.0.1:%d
listen.allowed_clients = 127.0.0.1

pm = dynamic
pm.max_children = 5
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3

; Security
security.limit_extensions = .php

; Enable status page for health checks
pm.status_path = /fpm-status
`, pidFile, errorLog, port)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return "", err
	}

	return configPath, nil
}

// monitorProcess watches for FPM process exit
func (fm *FPMManager) monitorProcess(version string, cmd *exec.Cmd) {
	cmd.Wait()

	fm.mu.Lock()
	if pool, exists := fm.pools[version]; exists {
		pool.Running = false
		pool.PID = 0
	}
	fm.mu.Unlock()

	fmt.Printf("ℹ️ PHP-FPM %s exited\n", version)
}

// killProcess terminates a process by PID
func (fm *FPMManager) killProcess(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	// Try SIGTERM first
	if err := process.Signal(syscall.SIGTERM); err == nil {
		return
	}

	// Force kill
	process.Signal(syscall.SIGKILL)
}

// savePID saves PID to file
func (fm *FPMManager) savePID(version string, pid int) {
	pidFile := filepath.Join(fm.pidDir, fmt.Sprintf("php-fpm-%s.pid", version))
	os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// loadPID loads PID from file
func (fm *FPMManager) loadPID(version string) int {
	pidFile := filepath.Join(fm.pidDir, fmt.Sprintf("php-fpm-%s.pid", version))
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

// removePID removes PID file
func (fm *FPMManager) removePID(version string) {
	pidFile := filepath.Join(fm.pidDir, fmt.Sprintf("php-fpm-%s.pid", version))
	os.Remove(pidFile)
}

// StopAll stops all running FPM pools
func (fm *FPMManager) StopAll() {
	fm.mu.RLock()
	versions := make([]string, 0, len(fm.pools))
	for v := range fm.pools {
		versions = append(versions, v)
	}
	fm.mu.RUnlock()

	for _, v := range versions {
		fm.StopFPM(v)
	}
}

// EnsureRunning starts FPM if not already running
func (fm *FPMManager) EnsureRunning(version string) error {
	if fm.IsRunning(version) {
		return nil
	}
	return fm.StartFPM(version)
}
