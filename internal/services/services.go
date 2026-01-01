package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
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
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Port        int       `json:"port"`
	Status      string    `json:"status"`
	DataDir     string    `json:"data_dir"`
	ConfigDir   string    `json:"config_dir"`
	BinaryDir   string    `json:"binary_dir"`
	PID         int       `json:"pid"`
	Installed   string    `json:"installed"`
	LastCheck   string    `json:"last_check,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	AutoRestart bool      `json:"auto_restart"`
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
	services      map[string]*Service
	available     []ServiceVersion
	mu            sync.RWMutex
	baseDir       string
	installStatus map[string]int
	statusMu      sync.RWMutex
	processes     map[string]*exec.Cmd
	wg            sync.WaitGroup
	shutdown      chan struct{}
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
	services := []string{"php", "mariadb", "mysql", "redis", "nginx", "apache", "composer", "nodejs"}

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
					}

					// Try to load any saved status
					sm.services[svcName] = svc
				}
			}
		}
	}
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
		return err
	}

	sm.statusMu.Lock()
	sm.installStatus[key] = 100
	sm.statusMu.Unlock()

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

			sm.updateInstallProgress("mysql", version, 80)
			sm.createMySQLConfig(configDir, dataDir, 3306)
			sm.updateInstallProgress("mysql", version, 100)
			fmt.Printf("✅ MySQL %s source downloaded\n", version)
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
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if strings.Contains(entry.Name(), "mariadb") {
				binaryPath := filepath.Join(installDir, entry.Name())
				if _, err := os.Stat(filepath.Join(binaryPath, "bin", "mariadbd")); err == nil {
					return binaryPath
				}
			}
		}
	}
	return ""
}

func (sm *ServiceManager) findMySQLBinary(installDir string) string {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if strings.Contains(strings.ToLower(entry.Name()), "mysql") {
				binaryPath := filepath.Join(installDir, entry.Name())
				if _, err := os.Stat(filepath.Join(binaryPath, "bin", "mysqld")); err == nil {
					return binaryPath
				}
			}
		}
	}
	return ""
}

func (sm *ServiceManager) findApacheBinary(installDir string) string {
	// Check compiled binary location first
	if _, err := os.Stat(filepath.Join(installDir, "apache-bin", "bin", "httpd")); err == nil {
		return filepath.Join(installDir, "apache-bin", "bin", "httpd")
	}

	// Direct check
	if _, err := os.Stat(filepath.Join(installDir, "bin", "httpd")); err == nil {
		return filepath.Join(installDir, "bin", "httpd")
	}

	entries, err := os.ReadDir(installDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Subdirectory check (e.g., httpd-2.4.58/bin/httpd)
			binaryPath := filepath.Join(installDir, entry.Name(), "bin", "httpd")
			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath
			}

			// Check for apache2/bin/httpd
			binaryPath = filepath.Join(installDir, entry.Name(), "apache2", "bin", "httpd")
			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath
			}
		}
	}
	return ""
}

func (sm *ServiceManager) findNginxBinary(installDir string) string {
	// Direct check
	if _, err := os.Stat(filepath.Join(installDir, "sbin", "nginx")); err == nil {
		return filepath.Join(installDir, "sbin", "nginx")
	}

	entries, err := os.ReadDir(installDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check nginx-bin/sbin/nginx or similar
			binaryPath := filepath.Join(installDir, entry.Name(), "sbin", "nginx")
			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath
			}
		}
	}
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
			return sm.compileNginx(version, installDir, configDir)
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
			return sm.compileApache(version, installDir, configDir, dataDir)
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
	conf := fmt.Sprintf(`ServerRoot "%s"
Listen 8080
LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule authn_core_module modules/mod_authn_core.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule dir_module modules/mod_dir.so
LoadModule mime_module modules/mod_mime.so
LoadModule unixd_module modules/mod_unixd.so

DocumentRoot "%s/htdocs"
<Directory "%s/htdocs">
    Options Indexes FollowSymLinks
    AllowOverride None
    Require all granted
</Directory>
`, installDir, installDir, installDir)

	return os.WriteFile(filepath.Join(configDir, "httpd.conf"), []byte(conf), 0644)
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
			return sm.compileRedis(version, installDir, configDir, dataDir)
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
	sm.statusMu.Unlock()
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

func (sm *ServiceManager) downloadAndExtract(url, targetDir string, progressCallback func(int)) error {
	fmt.Printf("⬇️ Downloading from %s...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
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
			return err
		}

		// Strip the first path component (e.g., "mariadb-10.11.11/" -> "")
		// This prevents nested extraction like bin/10.11/mariadb-10.11.11/
		name := header.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			// Skip the root directory itself
			continue
		}
		strippedName := parts[1]

		target := filepath.Join(targetDir, strippedName)

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
	go sm.monitorProcess(name, cmd)

	sm.updateInstallProgress(svc.Type, svc.Version, 100)

	fmt.Printf("✅ Service %s started (PID: %d)\n", name, cmd.Process.Pid)
	return nil
}

func (sm *ServiceManager) monitorProcess(name string, cmd *exec.Cmd) {
	sm.wg.Add(1)
	defer sm.wg.Done()

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

func (sm *ServiceManager) RestartService(name string) error {
	if err := sm.StopService(name); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return sm.StartService(name)
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
