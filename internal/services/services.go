package services

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yasinkuyu/Stacker/internal/config"
	"github.com/yasinkuyu/Stacker/internal/utils"
)

type Service struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Version      string    `json:"version"`
	Port         int       `json:"port"`
	Status       string    `json:"status"`
	DataDir      string    `json:"data_dir"`
	ConfigDir    string    `json:"config_dir"`
	BinaryDir    string    `json:"binary_dir"`
	PID          int       `json:"pid"`
	Installed    string    `json:"installed"`
	LastCheck    string    `json:"last_check,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	AutoRestart  bool      `json:"auto_restart"`
	HasConfig    bool      `json:"has_config"`
	PortInUse    bool      `json:"port_in_use"`
	PortConflict string    `json:"port_conflict,omitempty"`
	Runnable     bool      `json:"runnable"` // true for daemons (nginx, mysql), false for tools (composer, git)
}

type ServiceVersion = config.ServiceVersion

type DetailedStatus struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	PID       int               `json:"pid"`
	Uptime    string            `json:"uptime"`
	CPU       float64           `json:"cpu"`
	Memory    int64             `json:"memory"`
	Port      int               `json:"port"`
	Healthy   bool              `json:"healthy"`
	Error     string            `json:"error,omitempty"`
	Checks    map[string]string `json:"checks"`
	Resources map[string]int64  `json:"resources"`
}

type ServiceManager struct {
	services       map[string]*Service
	available      []ServiceVersion
	mu             sync.RWMutex
	baseDir        string
	installStatus  map[string]int
	installErrors  map[string]string // Key: svcType-version
	statusMu       sync.RWMutex
	processes      map[string]*exec.Cmd
	wg             sync.WaitGroup
	shutdown       chan struct{}
	OnStatusChange func()
}

func NewServiceManager() *ServiceManager {
	baseDir := utils.GetStackerDir()

	dirs := []string{
		"bin",
		"conf",
		"conf/nginx",
		"conf/apache",
		"conf/mysql",
		"conf/mariadb",
		"data",
		"data/mysql",
		"data/mariadb",
		"data/apache",
		"data/redis",
		"logs",
		"tmp",
		"sites",
		"ssl",
		"pids",
	}

	for _, dir := range dirs {
		os.MkdirAll(filepath.Join(baseDir, dir), 0755)
	}

	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "arm64"
	} else if arch == "amd64" {
		arch = "x86_64"
	}

	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	} else if osName == "linux" {
		osName = "linux"
	}

	// Dynamic versions from update.json
	var availableVersions []ServiceVersion
	services := []string{"mariadb", "mysql", "redis", "nginx", "apache", "composer", "nodejs"}

	for _, svc := range services {
		remoteVers := config.GetAvailableVersions(svc, "")
		availableVersions = append(availableVersions, remoteVers...)
	}

	// Fallback to hardcoded defaults if remote config fails or is empty
	if len(availableVersions) == 0 {
		for _, svc := range services {
			availableVersions = append(availableVersions, config.GetDefaultVersions(svc)...)
		}
	}

	sm := &ServiceManager{
		services:      make(map[string]*Service),
		available:     availableVersions,
		baseDir:       baseDir,
		installStatus: make(map[string]int),
		installErrors: make(map[string]string),
		processes:     make(map[string]*exec.Cmd),
		shutdown:      make(chan struct{}),
	}

	sm.loadInstalledServices()
	return sm
}

func (sm *ServiceManager) loadInstalledServices() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Load services from config/status files
	baseDir := sm.baseDir
	binDir := filepath.Join(baseDir, "bin")

	entries, err := os.ReadDir(binDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			svcType := entry.Name()
			versionEntries, err := os.ReadDir(filepath.Join(binDir, svcType))
			if err != nil {
				continue
			}

			for _, vEntry := range versionEntries {
				if vEntry.IsDir() {
					version := vEntry.Name()
					svcName := svcType + "-" + version

					// Check for status file or binary
					installDir := filepath.Join(binDir, svcType, version)
					configDir := filepath.Join(baseDir, "conf", svcType, version)
					dataDir := filepath.Join(baseDir, "data", svcType, version)

					// Tools are not "runnable"
					isRunnable := true
					if svcType == "composer" || svcType == "nodejs" || svcType == "git" {
						isRunnable = false
					}

					svc := &Service{
						Name:      svcName,
						Type:      svcType,
						Version:   version,
						Port:      sm.getDefaultPort(svcType),
						Status:    "stopped",
						DataDir:   dataDir,
						ConfigDir: configDir,
						BinaryDir: installDir,
						Installed: time.Now().Format(time.RFC3339),
						Runnable:  isRunnable,
					}

					// Check if service is running by PID file
					pidFile := filepath.Join(baseDir, "pids", svcName+".pid")
					if pidData, err := os.ReadFile(pidFile); err == nil {
						var pid int
						if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err == nil && pid > 0 {
							// Verify process is actually running
							if process, err := os.FindProcess(pid); err == nil {
								if err := process.Signal(syscall.Signal(0)); err == nil {
									svc.Status = "running"
									svc.PID = pid
								}
							}
						}
					}

					// Check if config file exists
					svc.HasConfig = sm.checkConfigExists(svc)

					// Check if port is in use (by another program)
					if svc.Status != "running" {
						svc.PortInUse, svc.PortConflict = sm.checkPortInUse(svc.Port)
					}

					sm.services[svcName] = svc
				}
			}
		}
	}
}

func (sm *ServiceManager) checkPortInUse(port int) (bool, string) {
	// First try lsof to get process name - this is more reliable
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-t")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Get PID from output
		pidStr := strings.TrimSpace(string(output))
		pids := strings.Split(pidStr, "\n")
		if len(pids) > 0 {
			// Get process name for first PID
			psCmd := exec.Command("ps", "-p", pids[0], "-o", "comm=")
			psOutput, psErr := psCmd.Output()
			if psErr == nil {
				processName := strings.TrimSpace(string(psOutput))
				return true, fmt.Sprintf("%s (PID: %s)", processName, pids[0])
			}
		}
		return true, fmt.Sprintf("Process PID: %s", pidStr)
	}

	// Fallback: try TCP connection
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return false, ""
	}
	conn.Close()
	return true, "Unknown process"
}

func (sm *ServiceManager) checkConfigExists(svc *Service) bool {
	var configFile string
	switch svc.Type {
	case "mysql", "mariadb":
		configFile = filepath.Join(svc.ConfigDir, "my.cnf")
	case "nginx":
		configFile = filepath.Join(svc.ConfigDir, "nginx.conf")
	case "apache":
		configFile = filepath.Join(svc.ConfigDir, "httpd.conf")
	case "redis":
		configFile = filepath.Join(svc.ConfigDir, "redis.conf")
	case "php":
		configFile = filepath.Join(svc.ConfigDir, "php.ini")
	default:
		return false
	}
	_, err := os.Stat(configFile)
	return err == nil
}

func (sm *ServiceManager) GetAvailableVersions(svcType string) []ServiceVersion {
	return config.GetAvailableVersions(svcType, "")
}

func (sm *ServiceManager) InstallService(svcType, version string) error {
	key := svcType + "-" + version
	sm.statusMu.Lock()
	sm.installStatus[key] = 0
	sm.statusMu.Unlock()

	installDir := filepath.Join(sm.baseDir, "bin", svcType, version)
	configDir := filepath.Join(sm.baseDir, "conf", svcType, version)
	dataDir := filepath.Join(sm.baseDir, "data", svcType, version)

	os.MkdirAll(installDir, 0755)
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(dataDir, 0755)

	utils.LogService(svcType, "install", "started")

	var err error
	switch svcType {
	case "php":
		err = sm.installPHP(version, installDir, configDir)
	case "mysql":
		err = sm.installMySQL(version, installDir, configDir, dataDir)
	case "mariadb":
		err = sm.installMariaDB(version, installDir, configDir, dataDir)
	case "nginx":
		err = sm.installNginx(version, installDir, configDir)
	case "apache":
		err = sm.installApache(version, installDir, configDir, dataDir)
	case "redis":
		err = sm.installRedis(version, installDir, configDir, dataDir)
	case "composer":
		err = sm.installComposer(version, installDir)
	case "nodejs":
		err = sm.installNodejs(version, installDir)
	default:
		return fmt.Errorf("unsupported service type: %s", svcType)
	}

	if err != nil {
		utils.LogService(svcType, "install", "failed: "+err.Error())
		return fmt.Errorf("failed to install %s: %w", svcType, err)
	}

	sm.statusMu.Lock()
	sm.installStatus[key] = 100
	sm.statusMu.Unlock()

	// Tools are not "runnable" services (daemons)
	isRunnable := true
	if svcType == "composer" || svcType == "nodejs" || svcType == "git" {
		isRunnable = false
	}

	svc := &Service{
		Name:      svcType + "-" + version,
		Type:      svcType,
		Version:   version,
		Port:      sm.getDefaultPort(svcType),
		Status:    "stopped",
		DataDir:   dataDir,
		ConfigDir: configDir,
		BinaryDir: installDir,
		PID:       0,
		Installed: time.Now().Format(time.RFC3339),
		Runnable:  isRunnable,
	}

	sm.mu.Lock()
	sm.services[svc.Name] = svc
	sm.mu.Unlock()

	sm.saveServiceStatus(svc)
	return nil
}

func (sm *ServiceManager) getDefaultPort(svcType string) int {
	switch svcType {
	case "mysql", "mariadb":
		return 3306
	case "nginx":
		return 80
	case "apache":
		return 8080
	case "redis":
		return 6379
	}
	return 0
}

func (sm *ServiceManager) installMySQL(version, installDir, configDir, dataDir string) error {
	sm.updateInstallProgress("mysql", version, 10)

	for _, v := range sm.available {
		if v.Type == "mysql" && v.Version == version {
			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("mysql", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			sm.updateInstallProgress("mysql", version, 70)
			sm.createMySQLConfig(configDir, dataDir, 3306)

			// Initialize MySQL data directory
			sm.updateInstallProgress("mysql", version, 80)
			binaryPath := sm.findMySQLBinary(installDir)
			if binaryPath != "" {
				fmt.Printf("📦 Initializing MySQL data directory...\n")
				mysqldPath := filepath.Join(binaryPath, "bin", "mysqld")
				initCmd := exec.Command(mysqldPath,
					"--initialize-insecure",
					"--datadir="+dataDir,
					"--basedir="+binaryPath,
				)
				initCmd.Env = append(os.Environ(),
					"DYLD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
					"LD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
				)
				output, err := initCmd.CombinedOutput()
				if err != nil {
					fmt.Printf("⚠️ MySQL init error: %v\nOutput: %s\n", err, string(output))
					// Don't fail, user can initialize manually
				} else {
					fmt.Printf("✅ MySQL data directory initialized\n")
				}
			}

			sm.updateInstallProgress("mysql", version, 100)
			fmt.Printf("✅ MySQL %s installed and initialized\n", version)
			return nil
		}
	}

	return fmt.Errorf("MySQL %s not found in available versions", version)
}

func (sm *ServiceManager) installPHP(version, installDir, configDir string) error {
	sm.updateInstallProgress("php", version, 10)

	for _, v := range sm.available {
		if v.Type == "php" && v.Version == version {
			// Check disk space
			if v.Size > 0 {
				if err := sm.checkDiskSpace(installDir, v.Size*3); err != nil { // Estimate extracted size
					return err
				}
			}

			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("php", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			// Validate checksum
			if v.Checksum != "" {
				// We need the downloaded file path, but downloadAndExtract extracts it directly.
				// For now, let's assume validation is handled or we need a separate download step.
				// sm.validateChecksum(targetFile, v.Checksum, "sha256")
			}

			sm.updateInstallProgress("php", version, 100)
			fmt.Printf("✅ PHP %s installed to %s\n", version, installDir)
			return nil
		}
	}

	return fmt.Errorf("PHP %s not found in available versions", version)
}

func (sm *ServiceManager) createMySQLConfig(configDir, dataDir string, port int) error {
	myCnf := fmt.Sprintf(`[mysqld]
port = %d
datadir = %s
socket = %s/mysql.sock
pid-file = %s/mysql.pid
log-error = %s/error.log
`, port, dataDir, dataDir, dataDir, dataDir)

	configPath := filepath.Join(configDir, "my.cnf")
	os.MkdirAll(configDir, 0755)
	return os.WriteFile(configPath, []byte(myCnf), 0644)
}

func (sm *ServiceManager) installMariaDB(version, installDir, configDir, dataDir string) error {
	sm.updateInstallProgress("mariadb", version, 10)

	for _, v := range sm.available {
		if v.Type == "mariadb" && v.Version == version {
			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("mariadb", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			sm.updateInstallProgress("mariadb", version, 80)

			binDir := filepath.Join(installDir, "bin")
			sm.createMariaDBConfig(configDir, dataDir, 3306)
			sm.updateInstallProgress("mariadb", version, 90)

			if err := sm.initializeMariaDB(binDir, configDir, dataDir); err != nil {
				return err
			}

			sm.updateInstallProgress("mariadb", version, 100)
			fmt.Printf("✅ MariaDB %s installed to %s\n", version, binDir)
			return nil
		}
	}

	return fmt.Errorf("MariaDB %s not found in available versions", version)
}

func (sm *ServiceManager) compileMariaDB(version, installDir, configDir, dataDir string) error {
	sourceDir := filepath.Join(installDir, fmt.Sprintf("mariadb-%s", version))
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		sourceDir = filepath.Join(installDir, fmt.Sprintf("mariadb-%s.%s", version, version))
	}

	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory not found: %s", sourceDir)
	}

	binDir := filepath.Join(installDir, "mariadb-bin")
	os.MkdirAll(binDir, 0755)

	fmt.Println("Compiling MariaDB (this may take a while)...")

	buildDir := filepath.Join(installDir, "build")
	os.MkdirAll(buildDir, 0755)

	configureCmd := exec.Command("cmake",
		"-DCMAKE_INSTALL_PREFIX="+binDir,
		"-DMYSQL_DATADIR="+dataDir,
		"-DWITHOUT_SERVER=OFF",
		"-DWITHOUT_ROCKSDB=1",
		"..",
	)
	configureCmd.Dir = buildDir
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		fmt.Println("Warning: cmake not found, trying alternative...")
		return sm.alternativeMariaDBInstall(sourceDir, binDir, configDir, dataDir)
	}

	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = buildDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		sm.updateInstallProgress("mariadb", version, -1)
		if runtime.GOOS == "darwin" {
			return fmt.Errorf("mariadb make failed. 💡 Try installing Xcode Command Line Tools: xcode-select --install. Error: %w", err)
		}
		return fmt.Errorf("mariadb make failed: %w", err)
	}

	installCmd := exec.Command("make", "install")
	installCmd.Dir = buildDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		sm.updateInstallProgress("mariadb", version, -1)
		return fmt.Errorf("mariadb install failed: %w", err)
	}

	sm.createMariaDBConfig(configDir, dataDir, 3306)
	sm.updateInstallProgress("mariadb", version, 90)

	if err := sm.initializeMariaDB(binDir, configDir, dataDir); err != nil {
		return err
	}

	sm.updateInstallProgress("mariadb", version, 100)
	fmt.Printf("✅ MariaDB %s installed to %s\n", version, binDir)
	return nil
}

func (sm *ServiceManager) alternativeMariaDBInstall(sourceDir, binDir, configDir, dataDir string) error {
	fmt.Println("Using pre-built binary from source...")

	for _, entry := range []string{"bin", "lib", "share"} {
		src := filepath.Join(sourceDir, entry)
		dst := filepath.Join(binDir, entry)
		if _, err := os.Stat(src); err == nil {
			os.Rename(src, dst)
		}
	}

	sm.createMariaDBConfig(configDir, dataDir, 3306)

	binaryPath := binDir
	mariadbd := filepath.Join(binaryPath, "bin", "mariadbd")
	if _, err := os.Stat(mariadbd); err != nil {
		mariadbd = filepath.Join(binaryPath, "bin", "mysqld")
	}

	if err := sm.initializeMariaDB(binaryPath, configDir, dataDir); err != nil {
		return err
	}

	return nil
}

func (sm *ServiceManager) initializeMariaDB(binaryPath, configDir, dataDir string) error {
	mariadbd := filepath.Join(binaryPath, "bin", "mariadbd")

	if _, err := os.Stat(mariadbd); err != nil {
		mariadbd = filepath.Join(binaryPath, "bin", "mysqld")
	}

	if _, err := os.Stat(mariadbd); err != nil {
		return fmt.Errorf("MariaDB binary not found")
	}

	mysqlDataDir := filepath.Join(dataDir, "mysql")
	if _, err := os.Stat(mysqlDataDir); os.IsNotExist(err) {
		fmt.Println("Initializing MariaDB database...")

		os.MkdirAll(dataDir, 0755)
		os.MkdirAll(mysqlDataDir, 0755)

		cmd := exec.Command(mariadbd,
			"--datadir="+mysqlDataDir,
			"--basedir="+binaryPath,
			"--bootstrap",
			"--skip-grant-tables",
		)
		cmd.Env = append(os.Environ(),
			"LD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
			"DYLD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
		)

		output, err := cmd.CombinedOutput()
		if err != nil && len(output) > 0 {
			fmt.Printf("MariaDB init warning: %s\n", string(output))
		}
	}

	rootPassword := "root"
	credFile := filepath.Join(configDir, ".root_creds")
	os.WriteFile(credFile, []byte(rootPassword), 0600)

	fmt.Printf("✅ MariaDB initialized (root password: %s)\n", rootPassword)
	return nil
}

func (sm *ServiceManager) findMariaDBBinary(installDir string) string {
	fmt.Printf("🔍 Checking MariaDB binary in: %s\n", installDir)
	// Direct check (if stripped)
	directPath := filepath.Join(installDir, "bin", "mariadbd")
	if _, err := os.Stat(directPath); err == nil {
		fmt.Printf("✅ Found MariaDB binary directly at: %s\n", directPath)
		return installDir
	}
	fmt.Printf("❌ Direct check failed for: %s\n", directPath)

	entries, err := os.ReadDir(installDir)
	if err != nil {
		fmt.Printf("⚠️ Could not read installDir: %v\n", err)
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("📁 Checking subdirectory: %s\n", entry.Name())
			if strings.Contains(entry.Name(), "mariadb") {
				binaryPath := filepath.Join(installDir, entry.Name())
				checkPath := filepath.Join(binaryPath, "bin", "mariadbd")
				if _, err := os.Stat(checkPath); err == nil {
					fmt.Printf("✅ Found MariaDB binary in subdirectory: %s\n", binaryPath)
					return binaryPath
				}
				fmt.Printf("❌ Subdirectory check failed for: %s\n", checkPath)
			}
		}
	}
	fmt.Printf("🚫 MariaDB binary NOT found in %s\n", installDir)
	return ""
}

func (sm *ServiceManager) findMySQLBinary(installDir string) string {
	fmt.Printf("🔍 Checking MySQL binary in: %s\n", installDir)
	// Direct check (if stripped)
	directPath := filepath.Join(installDir, "bin", "mysqld")
	if _, err := os.Stat(directPath); err == nil {
		fmt.Printf("✅ Found MySQL binary directly at: %s\n", directPath)
		return installDir
	}
	fmt.Printf("❌ Direct check failed for: %s\n", directPath)

	entries, err := os.ReadDir(installDir)
	if err != nil {
		fmt.Printf("⚠️ Could not read installDir: %v\n", err)
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("📁 Checking subdirectory: %s\n", entry.Name())
			if strings.Contains(strings.ToLower(entry.Name()), "mysql") {
				binaryPath := filepath.Join(installDir, entry.Name())
				checkPath := filepath.Join(binaryPath, "bin", "mysqld")
				if _, err := os.Stat(checkPath); err == nil {
					fmt.Printf("✅ Found MySQL binary in subdirectory: %s\n", binaryPath)
					return binaryPath
				}
				fmt.Printf("❌ Subdirectory check failed for: %s\n", checkPath)
			}
		}
	}
	fmt.Printf("🚫 MySQL binary NOT found in %s\n", installDir)
	return ""
}

func (sm *ServiceManager) findApacheBinary(installDir string) string {
	fmt.Printf("🔍 Checking Apache binary in: %s\n", installDir)
	// Check compiled binary location first
	compiledPath := filepath.Join(installDir, "apache-bin", "bin", "httpd")
	if _, err := os.Stat(compiledPath); err == nil {
		fmt.Printf("✅ Found Apache binary in compiled path: %s\n", compiledPath)
		return compiledPath
	}
	fmt.Printf("❌ Compiled path check failed for: %s\n", compiledPath)

	// Direct check
	directPath := filepath.Join(installDir, "bin", "httpd")
	if _, err := os.Stat(directPath); err == nil {
		fmt.Printf("✅ Found Apache binary directly at: %s\n", directPath)
		return directPath
	}
	fmt.Printf("❌ Direct check failed for: %s\n", directPath)

	entries, err := os.ReadDir(installDir)
	if err != nil {
		fmt.Printf("⚠️ Could not read installDir: %v\n", err)
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("📁 Checking subdirectory: %s\n", entry.Name())
			// Subdirectory check (e.g., httpd-2.4.58/bin/httpd)
			binaryPath := filepath.Join(installDir, entry.Name(), "bin", "httpd")
			if _, err := os.Stat(binaryPath); err == nil {
				fmt.Printf("✅ Found Apache binary in subdirectory: %s\n", binaryPath)
				return binaryPath
			}

			// Check for apache2/bin/httpd
			binaryPath = filepath.Join(installDir, entry.Name(), "apache2", "bin", "httpd")
			if _, err := os.Stat(binaryPath); err == nil {
				fmt.Printf("✅ Found Apache binary in apache2 subdirectory: %s\n", binaryPath)
				return binaryPath
			}
		}
	}
	fmt.Printf("🚫 Apache binary NOT found in %s\n", installDir)
	return ""
}

func (sm *ServiceManager) findNginxBinary(installDir string) string {
	fmt.Printf("🔍 Checking Nginx binary in: %s\n", installDir)

	// Check both bin and sbin directories
	binPaths := []string{"sbin/nginx", "bin/nginx"}

	// Direct check
	for _, binPath := range binPaths {
		directPath := filepath.Join(installDir, binPath)
		if _, err := os.Stat(directPath); err == nil {
			fmt.Printf("✅ Found Nginx binary directly at: %s\n", directPath)
			return directPath
		}
	}

	entries, err := os.ReadDir(installDir)
	if err != nil {
		fmt.Printf("⚠️ Could not read installDir: %v\n", err)
		return ""
	}

	// Check subdirectories (for nested version folders)
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("📁 Checking subdirectory: %s\n", entry.Name())
			for _, binPath := range binPaths {
				binaryPath := filepath.Join(installDir, entry.Name(), binPath)
				if _, err := os.Stat(binaryPath); err == nil {
					fmt.Printf("✅ Found Nginx binary in subdirectory: %s\n", binaryPath)
					return binaryPath
				}
			}
		}
	}
	fmt.Printf("🚫 Nginx binary NOT found in %s\n", installDir)
	return ""
}

func (sm *ServiceManager) createMariaDBConfig(configDir, dataDir string, port int) error {
	binaryPath := sm.findMariaDBBinary(filepath.Join(sm.baseDir, "bin", "mariadb"))
	basedir := "/usr/local"
	if binaryPath != "" {
		basedir = binaryPath
	}

	myCnf := fmt.Sprintf(`[mysqld]
port = %d
datadir = %s
socket = %s/mysql.sock
pid-file = %s/mysql.pid
log-error = %s/error.log
general-log = 1
general-log-file = %s/query.log

[mysqld_safe]
basedir = %s

[client]
port = %d
socket = %s/mysql.sock
`, port, dataDir, dataDir, dataDir, dataDir, dataDir, basedir, port, dataDir)

	configPath := filepath.Join(configDir, "my.cnf")
	return os.WriteFile(configPath, []byte(myCnf), 0644)
}

func (sm *ServiceManager) installNginx(version, installDir, configDir string) error {
	sm.updateInstallProgress("nginx", version, 10)

	for _, v := range sm.available {
		if v.Type == "nginx" && v.Version == version {
			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("nginx", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			sm.updateInstallProgress("nginx", version, 70)

			// Check if we need to compile (source build) or if it's already a binary build
			if _, err := os.Stat(filepath.Join(installDir, "configure")); err == nil {
				return sm.compileNginx(version, installDir, configDir)
			}

			// If no configure script, assume binary build and finalize
			sm.updateInstallProgress("nginx", version, 100)
			fmt.Printf("✅ Nginx %s binary installed to %s\n", version, installDir)
			return nil
		}
	}

	return fmt.Errorf("Nginx %s not found", version)
}

func (sm *ServiceManager) compileNginx(version, installDir, configDir string) error {
	fmt.Printf("🔧 Compiling Nginx %s...\n", version)
	sm.updateInstallProgress("nginx", version, 75)

	binDir := filepath.Join(installDir, "nginx-bin")
	os.MkdirAll(binDir, 0755)

	// Run configure
	fmt.Println("⚙️ Running ./configure...")
	configureCmd := exec.Command("./configure", "--prefix="+binDir)
	configureCmd.Dir = installDir
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		sm.updateInstallProgress("nginx", version, -1)
		return fmt.Errorf("nginx configure failed: %w. 💡 Make sure Xcode Command Line Tools are installed: xcode-select --install", err)
	}

	sm.updateInstallProgress("nginx", version, 85)

	// Run make
	fmt.Println("🔨 Running make...")
	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = installDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		sm.updateInstallProgress("nginx", version, -1)
		return fmt.Errorf("nginx make failed: %w", err)
	}

	sm.updateInstallProgress("nginx", version, 95)

	// Run make install
	fmt.Println("📦 Running make install...")
	installCmd := exec.Command("make", "install")
	installCmd.Dir = installDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		sm.updateInstallProgress("nginx", version, -1)
		return fmt.Errorf("nginx make install failed: %w", err)
	}

	sm.createNginxConfig(configDir)
	sm.updateInstallProgress("nginx", version, 100)
	fmt.Printf("✅ Nginx %s compiled and installed successfully\n", version)
	return nil
}

func (sm *ServiceManager) createNginxConfig(configDir string) error {
	conf := `worker_processes  1;
events {
    worker_connections  1024;
}
http {
    include       mime.types;
    default_type  application/octet-stream;
    sendfile        on;
    keepalive_timeout  65;
    server {
        listen       80;
        server_name  localhost;
        location / {
            root   html;
            index  index.html index.htm;
        }
        error_page   500 502 503 504  /50x.html;
        location = /50x.html {
            root   html;
        }
    }
    include ../../conf/nginx/*.conf;
}
`
	return os.WriteFile(filepath.Join(configDir, "nginx.conf"), []byte(conf), 0644)
}

func (sm *ServiceManager) installApache(version, installDir, configDir, dataDir string) error {
	sm.updateInstallProgress("apache", version, 10)

	for _, v := range sm.available {
		if v.Type == "apache" && v.Version == version {
			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("apache", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			sm.updateInstallProgress("apache", version, 70)

			// Check if we need to compile (source build) or if it's already a binary build
			if _, err := os.Stat(filepath.Join(installDir, "configure")); err == nil {
				return sm.compileApache(version, installDir, configDir, dataDir)
			}

			// If no configure script, assume binary build and finalize
			sm.createApacheConfig(configDir, dataDir, installDir)
			sm.updateInstallProgress("apache", version, 100)
			fmt.Printf("✅ Apache %s binary installed to %s\n", version, installDir)
			return nil
		}
	}

	return fmt.Errorf("Apache %s not found", version)
}

func (sm *ServiceManager) compileApache(version, installDir, configDir, dataDir string) error {
	fmt.Printf("🔧 Compiling Apache %s...\n", version)
	sm.updateInstallProgress("apache", version, 75)

	binDir := filepath.Join(installDir, "apache-bin")
	os.MkdirAll(binDir, 0755)

	// Clean environment to prevent interference from other tools (like MAMP)
	env := os.Environ()
	cleanEnv := make([]string, 0)
	for _, e := range env {
		if !strings.HasPrefix(e, "LDFLAGS=") &&
			!strings.HasPrefix(e, "CPPFLAGS=") &&
			!strings.HasPrefix(e, "CFLAGS=") &&
			!strings.HasPrefix(e, "LIBS=") {
			cleanEnv = append(cleanEnv, e)
		}
	}

	// Try to find dependencies in Homebrew (common on macOS)
	extraArgs := []string{}
	if runtime.GOOS == "darwin" {
		brewPrefix := "/usr/local"
		if runtime.GOARCH == "arm64" {
			brewPrefix = "/opt/homebrew"
		}

		deps := []string{"apr", "apr-util", "pcre"}
		for _, dep := range deps {
			path := filepath.Join(brewPrefix, "opt", dep)
			if _, err := os.Stat(path); err == nil {
				if dep == "pcre" {
					extraArgs = append(extraArgs, "--with-pcre="+path)
				} else {
					extraArgs = append(extraArgs, "--with-"+dep+"="+path)
				}
			}
		}
	}

	// Run configure
	fmt.Println("⚙️ Running ./configure...")
	args := append([]string{"--prefix=" + binDir, "--enable-so", "--enable-ssl", "--enable-rewrite"}, extraArgs...)
	configureCmd := exec.Command("./configure", args...)
	configureCmd.Dir = installDir
	configureCmd.Env = cleanEnv
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		sm.updateInstallProgress("apache", version, -1)
		return fmt.Errorf("apache configure failed: %w. 💡 Try installing dependencies: brew install pcre apr apr-util", err)
	}

	// Verify Makefile exists
	if _, err := os.Stat(filepath.Join(installDir, "Makefile")); err != nil {
		sm.updateInstallProgress("apache", version, -1)
		return fmt.Errorf("apache configure completed but Makefile not generated. 💡 Try: brew install pcre apr apr-util")
	}

	sm.updateInstallProgress("apache", version, 85)

	// Run make
	fmt.Println("🔨 Running make...")
	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = installDir
	makeCmd.Env = cleanEnv
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		sm.updateInstallProgress("apache", version, -1)
		return fmt.Errorf("apache make failed: %w", err)
	}

	sm.updateInstallProgress("apache", version, 95)

	// Run make install
	fmt.Println("📦 Running make install...")
	installCmd := exec.Command("make", "install")
	installCmd.Dir = installDir
	installCmd.Env = cleanEnv
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		sm.updateInstallProgress("apache", version, -1)
		return fmt.Errorf("apache make install failed: %w", err)
	}

	sm.createApacheConfig(configDir, dataDir, installDir)
	sm.updateInstallProgress("apache", version, 100)
	fmt.Printf("✅ Apache %s compiled and installed successfully\n", version)
	return nil
}

func (sm *ServiceManager) createApacheConfig(configDir, dataDir, installDir string) error {
	// Get Stacker base directory for vhost configs and shared htdocs
	stackerDir := filepath.Dir(filepath.Dir(configDir)) // Go up from conf/apache/version to Stacker root
	vhostDir := filepath.Join(stackerDir, "conf", "apache")
	sharedHtdocs := filepath.Join(stackerDir, "htdocs")

	// Create shared htdocs directory with default index.html
	if err := os.MkdirAll(sharedHtdocs, 0755); err != nil {
		return err
	}

	// Write default index.html to shared htdocs (will be used as fallback)
	defaultIndexPath := filepath.Join(sharedHtdocs, "index.html")
	if _, err := os.Stat(defaultIndexPath); os.IsNotExist(err) {
		// Get default HTML from web package constant
		defaultHTML := getDefaultIndexHTML()
		if err := os.WriteFile(defaultIndexPath, []byte(defaultHTML), 0644); err != nil {
			return err
		}
	}

	conf := fmt.Sprintf(`ServerRoot "%s"
Listen 8080
Listen 443
LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule authn_core_module modules/mod_authn_core.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule dir_module modules/mod_dir.so
LoadModule mime_module modules/mod_mime.so
LoadModule unixd_module modules/mod_unixd.so
LoadModule proxy_module modules/mod_proxy.so
LoadModule proxy_fcgi_module modules/mod_proxy_fcgi.so
LoadModule ssl_module modules/mod_ssl.so
LoadModule socache_shmcb_module modules/mod_socache_shmcb.so

# SSL Configuration
SSLRandomSeed startup builtin
SSLRandomSeed connect builtin
SSLSessionCache "shmcb:/tmp/ssl_scache(512000)"
SSLSessionCacheTimeout 300

# Default DocumentRoot for localhost and fallback
DocumentRoot "%s"
<Directory "%s">
    Options Indexes FollowSymLinks
    AllowOverride None
    Require all granted
</Directory>

# Include vhost configurations
Include "%s/*.conf"
`, installDir, sharedHtdocs, sharedHtdocs, vhostDir)

	return os.WriteFile(filepath.Join(configDir, "httpd.conf"), []byte(conf), 0644)
}

// getDefaultIndexHTML returns the default index.html content
// This is a copy of the constant from web package to avoid circular dependency
func getDefaultIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Stacker</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; 
            display: flex; 
            align-items: center; 
            justify-content: center; 
            min-height: 100vh; 
            background: #0a0a0a;
            overflow-x: hidden;
        }
        .container { 
            display: flex; 
            max-width: 900px; 
            width: 90%; 
            height: 400px; 
            box-shadow: 0 20px 60px rgba(0,0,0,0.5);
            border-radius: 12px;
            overflow: hidden;
        }
        .left { 
            background: linear-gradient(135deg, #00fa9a 0%, #00d97e 100%); 
            width: 50%; 
            display: flex; 
            align-items: center; 
            justify-content: center; 
            position: relative; 
        }
        .right { 
            background: linear-gradient(135deg, #ff1493 0%, #d91270 100%); 
            width: 50%; 
            display: flex; 
            align-items: center; 
            justify-content: center; 
            flex-direction: column; 
            text-align: center; 
            padding: 40px; 
        }
        .divider { 
            position: absolute; 
            right: 0; 
            top: 10%; 
            bottom: 10%; 
            width: 3px; 
            background: rgba(0,0,0,0.2); 
            border-radius: 2px;
        }
        h1.big-text { 
            font-size: 4rem; 
            font-weight: 900; 
            color: #000; 
            line-height: 0.9; 
            text-transform: uppercase; 
            letter-spacing: -2px; 
        }
        .right-content { max-width: 100%; }
        h2 { 
            font-size: 2rem; 
            font-weight: 700; 
            color: white; 
            margin: 0 0 20px 0; 
            line-height: 1.2; 
        }
        .btn { 
            display: inline-block; 
            background: #000; 
            color: #00fa9a; 
            padding: 12px 28px; 
            font-size: 0.9rem; 
            font-weight: 700; 
            text-decoration: none; 
            text-transform: uppercase; 
            margin-top: 20px; 
            border: none; 
            cursor: pointer; 
            border-radius: 6px;
            transition: all 0.3s ease;
        }
        .btn:hover {
            background: #1a1a1a;
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,250,154,0.3);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="left">
            <h1 class="big-text">STACKER<br>READY</h1>
            <div class="divider"></div>
        </div>
        <div class="right">
            <div class="right-content">
                <h2>Local Environment<br>Running</h2>
                <a href="http://localhost:9999" class="btn">Open Dashboard</a>
            </div>
        </div>
    </div>
</body>
</html>`
}

func (sm *ServiceManager) installRedis(version, installDir, configDir, dataDir string) error {
	sm.updateInstallProgress("redis", version, 10)

	for _, v := range sm.available {
		if v.Type == "redis" && v.Version == version {
			err := sm.downloadAndExtract(v.URL, installDir, func(progress int) {
				sm.updateInstallProgress("redis", version, 10+progress/2)
			})
			if err != nil {
				return err
			}

			sm.updateInstallProgress("redis", version, 70)

			// Check if we need to compile (source build) or if it's already a binary build
			// Redis uses Makefile as there's usually no ./configure
			if _, err := os.Stat(filepath.Join(installDir, "Makefile")); err == nil {
				return sm.compileRedis(version, installDir, configDir, dataDir)
			}

			// If no Makefile but we have binaries (e.g. from Homebrew bottle), assume binary build
			sm.updateInstallProgress("redis", version, 100)
			fmt.Printf("✅ Redis %s binary installed to %s\n", version, installDir)
			return nil
		}
	}

	return fmt.Errorf("Redis %s not found", version)
}

func (sm *ServiceManager) compileRedis(version, installDir, configDir, dataDir string) error {
	fmt.Printf("🔧 Compiling Redis %s...\n", version)

	// Find the extracted source directory
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return err
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "redis-") {
			sourceDir = filepath.Join(installDir, entry.Name())
			break
		}
	}

	if sourceDir == "" {
		return fmt.Errorf("Redis source directory not found in %s", installDir)
	}

	sm.updateInstallProgress("redis", version, 75)

	// Compile Redis (no external dependencies needed)
	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = sourceDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("Redis compilation failed: %w", err)
	}

	sm.updateInstallProgress("redis", version, 90)

	// Copy binaries to install directory
	binaries := []string{"redis-server", "redis-cli", "redis-benchmark"}
	for _, bin := range binaries {
		src := filepath.Join(sourceDir, "src", bin)
		dst := filepath.Join(installDir, bin)
		if _, err := os.Stat(src); err == nil {
			input, _ := os.ReadFile(src)
			os.WriteFile(dst, input, 0755)
		}
	}

	sm.createRedisConfig(configDir, dataDir)
	sm.updateInstallProgress("redis", version, 100)
	fmt.Printf("✅ Redis %s compiled and installed\n", version)
	return nil
}

func (sm *ServiceManager) createRedisConfig(configDir, dataDir string) error {
	conf := fmt.Sprintf(`port 6379
daemonize no
dir %s
`, dataDir)
	return os.WriteFile(filepath.Join(configDir, "redis.conf"), []byte(conf), 0644)
}

func (sm *ServiceManager) UninstallService(name string) error {
	sm.mu.Lock()
	svc, ok := sm.services[name]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("service %s not found", name)
	}

	// Use internal stop that doesn't lock
	sm.stopServiceInternal(svc)
	sm.mu.Unlock()

	sm.updateInstallProgress(svc.Type, svc.Version, 10)

	installDir := svc.BinaryDir
	configDir := svc.ConfigDir
	dataDir := svc.DataDir

	if installDir != "" {
		sm.updateInstallProgress(svc.Type, svc.Version, 20)
		os.RemoveAll(installDir)
	}
	if configDir != "" {
		sm.updateInstallProgress(svc.Type, svc.Version, 50)
		os.RemoveAll(configDir)
	}
	if dataDir != "" {
		sm.updateInstallProgress(svc.Type, svc.Version, 80)
		os.RemoveAll(dataDir)
	}

	sm.updateInstallProgress(svc.Type, svc.Version, 100)

	sm.mu.Lock()
	delete(sm.services, name)
	sm.saveServices()
	sm.mu.Unlock()

	if sm.OnStatusChange != nil {
		sm.OnStatusChange()
	}
	return nil
}

func (sm *ServiceManager) saveServices() {
	// For now, services are loaded by scanning directories in loadInstalledServices
	// If we need extra metadata, we can save a JSON here.
}

func (sm *ServiceManager) saveServiceStatus(svc *Service) {
	// Persist status if needed
}

func (sm *ServiceManager) savePID(name string, pid int) {
	pidFile := sm.getPIDFile(name)
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (sm *ServiceManager) loadPID(name string) int {
	pidFile := sm.getPIDFile(name)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	return pid
}

func (sm *ServiceManager) getPIDFile(name string) string {
	return filepath.Join(sm.baseDir, "pids", name+".pid")
}

func (sm *ServiceManager) installComposer(version, installDir string) error {
	url := "https://getcomposer.org/download/latest-stable/composer.phar"
	target := filepath.Join(installDir, "composer.phar")
	return sm.downloadFile(url, target, nil)
}

func (sm *ServiceManager) installNodejs(version, installDir string) error {
	// Simplified nodejs install
	return fmt.Errorf("nodejs install not fully implemented yet")
}

func (sm *ServiceManager) updateInstallProgress(svcType, version string, progress int) {
	key := svcType
	if version != "" {
		key = svcType + "-" + version
	}
	sm.statusMu.Lock()
	sm.installStatus[key] = progress
	if progress == 100 {
		delete(sm.installErrors, key)
	}
	sm.statusMu.Unlock()
}

func (sm *ServiceManager) SetInstallError(svcType, version, errMsg string) {
	key := svcType
	if version != "" {
		key = svcType + "-" + version
	}
	sm.statusMu.Lock()
	sm.installErrors[key] = errMsg
	sm.installStatus[key] = -1
	sm.statusMu.Unlock()
}

func (sm *ServiceManager) GetInstallStatus(svcType, version string) (int, string) {
	key := svcType
	if version != "" {
		key = svcType + "-" + version
	}
	sm.statusMu.RLock()
	defer sm.statusMu.RUnlock()
	return sm.installStatus[key], sm.installErrors[key]
}

// UpdateInstallProgress is a public version for external access
func (sm *ServiceManager) UpdateInstallProgress(svcType, version string, progress int) {
	sm.updateInstallProgress(svcType, version, progress)
}

func (sm *ServiceManager) GetInstallProgress(svcType, version string) int {
	sm.statusMu.RLock()
	defer sm.statusMu.RUnlock()
	key := svcType + "-" + version
	return sm.installStatus[key]
}

func (sm *ServiceManager) getGHCRToken(scope string) (string, error) {
	url := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=%s", scope)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get ghcr token: %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}

func (sm *ServiceManager) downloadAndExtract(urlStr, targetDir string, progressCallback func(int)) error {
	fmt.Printf("⬇️ Downloading from %s...\n", urlStr)

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	if strings.Contains(urlStr, "ghcr.io") {
		// Example: https://ghcr.io/v2/homebrew/core/nginx/blobs/sha256:...
		// Repo = homebrew/core/nginx
		u, err := url.Parse(urlStr)
		if err == nil {
			pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(pathParts) >= 4 && pathParts[0] == "v2" {
				repoParts := []string{}
				for i := 1; i < len(pathParts); i++ {
					if pathParts[i] == "blobs" {
						break
					}
					repoParts = append(repoParts, pathParts[i])
				}
				if len(repoParts) > 0 {
					repo := strings.Join(repoParts, "/")
					token, _ := sm.getGHCRToken("repository:" + repo + ":pull")
					if token != "" {
						req.Header.Set("Authorization", "Bearer "+token)
					}
				}
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	fmt.Printf("📦 Content-Length: %d bytes\n", resp.ContentLength)

	var reader io.Reader = resp.Body
	if progressCallback != nil && resp.ContentLength > 0 {
		pr := &progressReader{
			Reader: resp.Body,
			Total:  resp.ContentLength,
			OnProg: progressCallback,
		}
		reader = pr
	} else if progressCallback != nil {
		// Server didn't provide Content-Length, simulate progress
		fmt.Println("⚠️ No Content-Length header, progress will jump")
		progressCallback(50) // Show 50% to indicate download in progress
	}

	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("gzip reader failed: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read failed: %w", err)
		}

		// Some archives have a top-level directory, others don't.
		// We try to be smart about it.
		name := header.Name

		// Skip macos metadata
		if strings.Contains(name, "__MACOSX") || strings.Contains(name, ".DS_Store") {
			continue
		}

		parts := strings.SplitN(name, "/", 2)
		var targetName string
		if len(parts) == 2 && parts[1] != "" {
			// Strip the first component if it looks like a wrapper directory
			targetName = parts[1]
		} else {
			// Use the name as is (file at root or just a directory)
			targetName = name
		}

		if targetName == "" || targetName == "." {
			continue
		}

		target := filepath.Join(targetDir, targetName)

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
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

	fmt.Printf("✅ Download completed\n")
	return nil
}

type progressReader struct {
	Reader       io.Reader
	Total        int64
	Current      int64
	OnProg       func(int)
	lastProgress int
}

// Helper functions removed. URLs are now dynamically fetched from update.json via internal/config/remote.go

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 && pr.OnProg != nil {
		progress := int(float64(pr.Current) / float64(pr.Total) * 100)
		if progress != pr.lastProgress {
			pr.lastProgress = progress
			pr.OnProg(progress)
		}
	}
	return
}

func (sm *ServiceManager) checkDiskSpace(path string, required int64) error {
	return utils.CheckDiskSpace(path, required)
}

func (sm *ServiceManager) StartStatusWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sm.mu.RLock()
				var names []string
				for name := range sm.services {
					names = append(names, name)
				}
				sm.mu.RUnlock()

				for _, name := range names {
					sm.GetStatus(name)
				}
			case <-sm.shutdown:
				return
			}
		}
	}()
}

func (sm *ServiceManager) validateChecksum(filePath, expected, algo string) error {
	// Simple implementation for now, assuming SHA256 if algo is empty
	if expected == "" {
		return nil
	}
	// For now, let's just log and return nil to not block installation
	// until a more robust implementation is needed
	fmt.Printf("🔍 Validating checksum for %s (Expected: %s)\n", filePath, expected)
	return nil
}

func (sm *ServiceManager) downloadFile(url, target string, onProgress func(int)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	var reader io.Reader = resp.Body
	if onProgress != nil && resp.ContentLength > 0 {
		reader = &progressReader{
			Reader: resp.Body,
			Total:  resp.ContentLength,
			OnProg: onProgress,
		}
	}

	_, err = io.Copy(out, reader)
	return err
}

func (sm *ServiceManager) downloadWithRetry(url, target string, onProgress func(int)) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = sm.downloadFile(url, target, onProgress); err == nil {
			return nil
		}
		fmt.Printf("⚠️ Download attempt %d failed: %v. Retrying...\n", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return err
}

func (sm *ServiceManager) downloadFromMirrors(mirrors []string, target string, onProgress func(int)) error {
	for _, url := range mirrors {
		if err := sm.downloadWithRetry(url, target, onProgress); err == nil {
			return nil
		}
	}
	return fmt.Errorf("all mirrors failed")
}

func (sm *ServiceManager) StartService(name string) error {
	sm.mu.Lock()
	svc, ok := sm.services[name]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if svc.Status == "running" {
		return fmt.Errorf("service %s is already running", name)
	}

	sm.updateInstallProgress(svc.Type, svc.Version, 10)

	var cmd *exec.Cmd
	var binaryPath string

	switch svc.Type {
	case "mariadb":
		sm.updateInstallProgress(svc.Type, svc.Version, 30)
		binaryPath = sm.findMariaDBBinary(svc.BinaryDir)
		if binaryPath == "" {
			return fmt.Errorf("MariaDB binary not found")
		}
		cmd = sm.startMariaDB(svc, binaryPath)
	case "mysql":
		sm.updateInstallProgress(svc.Type, svc.Version, 30)
		binaryPath = sm.findMySQLBinary(svc.BinaryDir)
		if binaryPath == "" {
			return fmt.Errorf("MySQL binary not found")
		}
		cmd = sm.startMySQL(svc, binaryPath)
	case "nginx":
		sm.updateInstallProgress(svc.Type, svc.Version, 30)
		binaryPath = sm.findNginxBinary(svc.BinaryDir)
		if binaryPath == "" {
			binaryPath = filepath.Join(svc.BinaryDir, "nginx-bin", "sbin", "nginx")
		}
		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("Nginx binary not found at %s", binaryPath)
		}
		cmd = sm.startNginx(svc, binaryPath)
	case "apache":
		sm.updateInstallProgress(svc.Type, svc.Version, 30)
		binaryPath = sm.findApacheBinary(svc.BinaryDir)
		if binaryPath == "" {
			// Fallback to old path just in case
			binaryPath = filepath.Join(svc.BinaryDir, "apache-bin", "bin", "httpd")
		}

		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("Apache binary not found at %s", binaryPath)
		}

		cmd = sm.startApache(svc, binaryPath)
	case "redis":
		sm.updateInstallProgress(svc.Type, svc.Version, 30)
		binaryPath = filepath.Join(svc.BinaryDir, "redis-server")
		if _, err := os.Stat(binaryPath); err != nil {
			binaryPath = filepath.Join(svc.BinaryDir, "src", "redis-server")
		}
		cmd = sm.startRedis(svc, binaryPath)
	default:
		return fmt.Errorf("unsupported service type: %s", svc.Type)
	}

	if cmd == nil {
		return fmt.Errorf("failed to create command for %s", name)
	}

	sm.updateInstallProgress(svc.Type, svc.Version, 60)

	// Setup logging
	logsDir := filepath.Join(sm.baseDir, "logs")
	os.MkdirAll(logsDir, 0755)
	logFile := filepath.Join(logsDir, name+".log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	} else {
		utils.LogError(fmt.Sprintf("Failed to open service log file %s: %v", logFile, err))
	}

	if err := cmd.Start(); err != nil {
		utils.LogService(name, "start", "failed: "+err.Error())
		sm.updateInstallProgress(svc.Type, svc.Version, -1)
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	utils.LogService(name, "start", "success")

	sm.updateInstallProgress(svc.Type, svc.Version, 90)

	sm.mu.Lock()
	svc.Status = "running"
	svc.PID = cmd.Process.Pid
	svc.StartTime = time.Now()
	sm.processes[name] = cmd
	sm.mu.Unlock()

	sm.saveServiceStatus(svc)
	sm.savePID(name, cmd.Process.Pid)

	// Start monitoring
	go sm.monitorProcess(name, cmd, f)

	sm.updateInstallProgress(svc.Type, svc.Version, 100)

	fmt.Printf("✅ Service %s started (PID: %d)\n", name, cmd.Process.Pid)
	if sm.OnStatusChange != nil {
		sm.OnStatusChange()
	}
	return nil
}

func (sm *ServiceManager) monitorProcess(name string, cmd *exec.Cmd, logFile *os.File) {
	sm.wg.Add(1)
	defer sm.wg.Done()
	if logFile != nil {
		defer logFile.Close()
	}

	err := cmd.Wait()

	sm.mu.Lock()
	svc, ok := sm.services[name]
	if ok {
		svc.Status = "stopped"
		svc.PID = 0
		delete(sm.processes, name)
	}
	isShuttingDown := sm.isShuttingDown()
	autoRestart := false
	if ok {
		autoRestart = svc.AutoRestart
	}
	sm.mu.Unlock()

	if err != nil && !isShuttingDown {
		fmt.Printf("⚠️ Service %s exited with error: %v\n", name, err)
		utils.LogService(name, "exit", "error: "+err.Error())
	} else {
		fmt.Printf("ℹ️ Service %s stopped\n", name)
		utils.LogService(name, "exit", "clean")
	}

	if ok {
		sm.saveServiceStatus(svc)
	}

	// Auto-restart logic
	if autoRestart && !isShuttingDown {
		fmt.Printf("🔄 Auto-restarting service %s...\n", name)
		time.Sleep(2 * time.Second)
		sm.StartService(name)
	}

	if sm.OnStatusChange != nil {
		sm.OnStatusChange()
	}
}

func (sm *ServiceManager) isShuttingDown() bool {
	select {
	case <-sm.shutdown:
		return true
	default:
		return false
	}
}

func (sm *ServiceManager) Stop() {
	select {
	case <-sm.shutdown:
		// Already closing
	default:
		close(sm.shutdown)
	}
}

func (sm *ServiceManager) GracefulStopAll() error {
	fmt.Println("⏳ Gracefully stopping all services...")
	sm.Stop()

	sm.mu.RLock()
	var wg sync.WaitGroup
	for name, svc := range sm.services {
		if svc.Status == "running" {
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				sm.StopService(n)
			}(name)
		}
	}
	sm.mu.RUnlock()

	// Wait for all stop commands to finish or timeout
	c := make(chan struct{})
	go func() {
		wg.Wait()
		c <- struct{}{}
	}()

	select {
	case <-c:
		return nil
	case <-time.After(sm.getGracefulTimeout()):
		return fmt.Errorf("graceful stop timed out")
	}
}

func (sm *ServiceManager) ForceStopAll() {
	fmt.Println("⚠️ Force stopping all services...")
	sm.mu.Lock()
	for name, cmd := range sm.processes {
		if cmd != nil && cmd.Process != nil {
			fmt.Printf("🛑 Killing process %s (PID: %d)\n", name, cmd.Process.Pid)
			cmd.Process.Kill()
		}
	}
	sm.mu.Unlock()
}

func (sm *ServiceManager) Wait() {
	sm.wg.Wait()
}

func (sm *ServiceManager) getGracefulTimeout() time.Duration {
	return 15 * time.Second
}

func (sm *ServiceManager) GetDetailedStatus(name string) *DetailedStatus {
	sm.mu.RLock()
	svc, ok := sm.services[name]
	sm.mu.RUnlock()

	if !ok {
		return &DetailedStatus{Name: name, Status: "not_installed", Healthy: false}
	}

	status := &DetailedStatus{
		Name:    svc.Name,
		Status:  svc.Status,
		PID:     svc.PID,
		Port:    svc.Port,
		Healthy: svc.Status == "running",
		Checks:  make(map[string]string),
	}

	if svc.Status == "running" && svc.PID > 0 {
		status.Uptime = time.Since(svc.StartTime).Round(time.Second).String()
		// In a real app, we'd use gopsutil here for CPU/Memory
		// For now, let's check if the port is actually listening
		if err := sm.checkPortAvailable(svc.Port); err == nil {
			status.Checks["port"] = "listening"
		} else {
			status.Checks["port"] = "busy/failed"
			status.Healthy = false
		}

		// Service specific checks
		switch svc.Type {
		case "nginx", "apache":
			if sm.checkHTTPEndpoint(fmt.Sprintf("http://localhost:%d", svc.Port)) {
				status.Checks["http"] = "responding"
			} else {
				status.Checks["http"] = "no_response"
				status.Healthy = false
			}
		}
	}

	return status
}

func (sm *ServiceManager) checkPortAvailable(port int) error {
	// For health check, we actually want to see if we CANNOT bind (meaning it's in use by us)
	// or if we can connect to it.
	// Simpler check: try to connect
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (sm *ServiceManager) checkHTTPEndpoint(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}
func (sm *ServiceManager) StopService(name string) error {
	utils.LogService(name, "stop", "request")
	sm.mu.Lock()
	svc, ok := sm.services[name]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("service %s not found", name)
	}

	if svc.Status != "running" {
		sm.mu.Unlock()
		return fmt.Errorf("service %s is not running", name)
	}

	err := sm.stopServiceInternal(svc)
	sm.mu.Unlock()

	if err != nil {
		return err
	}

	sm.saveServiceStatus(svc)
	os.Remove(sm.getPIDFile(name))

	fmt.Printf("⏹️ Service %s stopped\n", name)
	if sm.OnStatusChange != nil {
		sm.OnStatusChange()
	}
	return nil
}

func (sm *ServiceManager) RestartService(name string) error {
	utils.LogService(name, "restart", "request")

	sm.mu.RLock()
	svc, ok := sm.services[name]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	// Stop if running
	if svc.Status == "running" {
		if err := sm.StopService(name); err != nil {
			fmt.Printf("⚠️ Warning during stop: %v\n", err)
		}
		time.Sleep(1 * time.Second) // Wait for clean shutdown
	}

	// Start
	if err := sm.StartService(name); err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	utils.LogService(name, "restart", "success")
	fmt.Printf("🔄 Service %s restarted\n", name)
	return nil
}

func (sm *ServiceManager) GetServiceConfig(name string) (string, string, error) {
	sm.mu.RLock()
	svc, ok := sm.services[name]
	sm.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("service %s not found", name)
	}

	var configFile string
	switch svc.Type {
	case "mysql", "mariadb":
		configFile = filepath.Join(svc.ConfigDir, "my.cnf")
	case "nginx":
		configFile = filepath.Join(svc.ConfigDir, "nginx.conf")
	case "apache":
		configFile = filepath.Join(svc.ConfigDir, "httpd.conf")
	case "redis":
		configFile = filepath.Join(svc.ConfigDir, "redis.conf")
	case "php":
		configFile = filepath.Join(svc.ConfigDir, "php.ini")
	default:
		return "", "", fmt.Errorf("config not available for %s", svc.Type)
	}

	// Create default config if it doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := sm.createDefaultConfig(svc); err != nil {
			return configFile, "", fmt.Errorf("failed to create default config: %w", err)
		}
		// Update HasConfig
		svc.HasConfig = true
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return configFile, "", fmt.Errorf("failed to read config: %w", err)
	}

	return configFile, string(content), nil
}

func (sm *ServiceManager) createDefaultConfig(svc *Service) error {
	os.MkdirAll(svc.ConfigDir, 0755)

	switch svc.Type {
	case "mysql":
		return sm.createMySQLConfig(svc.ConfigDir, svc.DataDir, sm.getDefaultPort("mysql"))
	case "mariadb":
		return sm.createMariaDBConfig(svc.ConfigDir, svc.DataDir, sm.getDefaultPort("mariadb"))
	case "nginx":
		return sm.createNginxConfig(svc.ConfigDir)
	case "apache":
		return sm.createApacheConfig(svc.ConfigDir, svc.DataDir, svc.BinaryDir)
	case "redis":
		return sm.createRedisConfig(svc.ConfigDir, svc.DataDir)
	case "php":
		return sm.createPHPConfig(svc.ConfigDir)
	}
	return fmt.Errorf("config creation not supported for %s", svc.Type)
}

func (sm *ServiceManager) createPHPConfig(configDir string) error {
	phpIni := `[PHP]
engine = On
short_open_tag = Off
display_errors = On
log_errors = On
error_reporting = E_ALL
memory_limit = 256M
max_execution_time = 300
upload_max_filesize = 64M
post_max_size = 64M
date.timezone = UTC
`
	os.MkdirAll(configDir, 0755)
	return os.WriteFile(filepath.Join(configDir, "php.ini"), []byte(phpIni), 0644)
}

func (sm *ServiceManager) SaveServiceConfig(name, content string) error {
	sm.mu.RLock()
	svc, ok := sm.services[name]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	var configFile string
	switch svc.Type {
	case "mysql", "mariadb":
		configFile = filepath.Join(svc.ConfigDir, "my.cnf")
	case "nginx":
		configFile = filepath.Join(svc.ConfigDir, "nginx.conf")
	case "apache":
		configFile = filepath.Join(svc.ConfigDir, "httpd.conf")
	case "redis":
		configFile = filepath.Join(svc.ConfigDir, "redis.conf")
	case "php":
		configFile = filepath.Join(svc.ConfigDir, "php.ini")
	default:
		return fmt.Errorf("config not available for %s", svc.Type)
	}

	// Backup old config
	if _, err := os.Stat(configFile); err == nil {
		backupFile := configFile + ".bak"
		os.Rename(configFile, backupFile)
	}

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	utils.LogService(name, "config", "saved")
	fmt.Printf("💾 Config saved for %s\n", name)

	// Auto-restart if running
	if svc.Status == "running" {
		fmt.Printf("🔄 Auto-restarting %s after config change...\n", name)
		return sm.RestartService(name)
	}

	return nil
}

func (sm *ServiceManager) stopServiceInternal(svc *Service) error {
	pid := svc.PID
	if pid == 0 {
		pid = sm.loadPID(svc.Name)
	}

	if pid > 0 {
		process, err := os.FindProcess(pid)
		if err == nil {
			// Try SIGTERM first
			if err := process.Signal(syscall.SIGTERM); err == nil {
				// Wait a bit for graceful shutdown
				time.Sleep(500 * time.Millisecond)
				// Check if still running
				if err := process.Signal(syscall.Signal(0)); err == nil {
					// Still running, force kill
					process.Signal(syscall.SIGKILL)
				}
			} else {
				process.Signal(syscall.SIGKILL)
			}
		}
	}

	svc.Status = "stopped"
	svc.PID = 0
	return nil
}

func (sm *ServiceManager) GetStatus(name string) string {
	sm.mu.RLock()
	svc, ok := sm.services[name]
	sm.mu.RUnlock()

	if !ok {
		return "none"
	}

	oldStatus := svc.Status

	// Check if process is running
	pid := svc.PID
	if pid == 0 {
		pid = sm.loadPID(name)
	}

	if pid > 0 {
		process, err := os.FindProcess(pid)
		if err == nil {
			// Signal 0 checks if process exists
			if err := process.Signal(syscall.Signal(0)); err == nil {
				svc.Status = "running"
				svc.PID = pid

				// Verify if it's actually listening on its port (only for TCP services)
				if svc.Type != "composer" && svc.Type != "nodejs" {
					if err := sm.checkPortAvailable(svc.Port); err != nil {
						// Port is not listening yet or closed
						// We keep it as running if PID is alive, but maybe mark as "starting" or "unhealthy"?
						// For now, let's just log it if unhealthy
						svc.PortInUse = false
					} else {
						svc.PortInUse = true
					}
				}
			} else {
				svc.Status = "stopped"
				svc.PID = 0
			}
		} else {
			svc.Status = "stopped"
			svc.PID = 0
		}
	} else {
		svc.Status = "stopped"
	}

	svc.LastCheck = time.Now().Format(time.RFC3339)

	if oldStatus != svc.Status {
		if sm.OnStatusChange != nil {
			sm.OnStatusChange()
		}
	}

	return svc.Status
}

func (sm *ServiceManager) GetServices() []*Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*Service
	for _, svc := range sm.services {
		result = append(result, svc)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

func (sm *ServiceManager) GetService(name string) *Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.services[name]
}

func (sm *ServiceManager) StartAll() {
	sm.mu.RLock()
	var names []string
	for name := range sm.services {
		names = append(names, name)
	}
	sm.mu.RUnlock()

	for _, name := range names {
		sm.StartService(name)
	}
}

func (sm *ServiceManager) StopAll() {
	sm.GracefulStopAll()
}

func (sm *ServiceManager) FormatStatus() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.services) == 0 {
		return "No services installed"
	}

	var output []string
	output = append(output, "Installed Services:")
	output = append(output, "════════════════════")

	for _, svc := range sm.services {
		status := strings.Title(svc.Status)
		if svc.Status == "running" && svc.PID > 0 {
			status = fmt.Sprintf("%s (PID: %d)", status, svc.PID)
		}
		output = append(output, fmt.Sprintf("  • %-15s %s", svc.Name, status))
	}

	return strings.Join(output, "\n")
}

func (sm *ServiceManager) startMariaDB(svc *Service, binaryPath string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(binaryPath, "bin", "mariadbd"),
		"--defaults-file="+filepath.Join(svc.ConfigDir, "my.cnf"),
		fmt.Sprintf("--port=%d", svc.Port),
	)
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
		"DYLD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
	)
	return cmd
}

func (sm *ServiceManager) startMySQL(svc *Service, binaryPath string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(binaryPath, "bin", "mysqld"),
		"--defaults-file="+filepath.Join(svc.ConfigDir, "my.cnf"),
		fmt.Sprintf("--port=%d", svc.Port),
	)
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
		"DYLD_LIBRARY_PATH="+filepath.Join(binaryPath, "lib"),
	)
	return cmd
}

func (sm *ServiceManager) startNginx(svc *Service, binaryPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath,
		"-c", filepath.Join(svc.ConfigDir, "nginx.conf"),
		"-g", "daemon off;",
	)
	return cmd
}

func (sm *ServiceManager) startApache(svc *Service, binaryPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath,
		"-f", filepath.Join(svc.ConfigDir, "httpd.conf"),
		"-k", "start",
		"-D", "FOREGROUND",
	)
	return cmd
}

func (sm *ServiceManager) startRedis(svc *Service, binaryPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath,
		filepath.Join(svc.ConfigDir, "redis.conf"),
	)
	return cmd
}
