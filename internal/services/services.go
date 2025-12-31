package services

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yasinkuyu/Stacker/internal/utils"
)

type Service struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	DataDir   string `json:"data_dir"`
	ConfigDir string `json:"config_dir"`
	BinaryDir string `json:"binary_dir"`
	PID       int    `json:"pid"`
	Installed string `json:"installed"`
}

type ServiceVersion struct {
	Type      string `json:"type"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
	Arch      string `json:"arch"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
}

type ServiceManager struct {
	services      map[string]*Service
	available     []ServiceVersion
	mu            sync.RWMutex
	baseDir       string
	installStatus map[string]int
	statusMu      sync.RWMutex
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

	availableVersions := []ServiceVersion{
		// MariaDB (source - requires compilation)
		{Type: "mariadb", Version: "11.2", Available: true, Arch: "all", URL: "https://downloads.mariadb.com/MariaDB/mariadb-11.2.2/source/mariadb-11.2.2.tar.gz"},
		{Type: "mariadb", Version: "10.11", Available: true, Arch: "all", URL: "https://downloads.mariadb.com/MariaDB/mariadb-10.11.8/source/mariadb-10.11.8.tar.gz"},
		{Type: "mariadb", Version: "10.6", Available: true, Arch: "all", URL: "https://downloads.mariadb.com/MariaDB/mariadb-10.6.19/source/mariadb-10.6.19.tar.gz"},

		// MySQL (source - requires compilation)
		{Type: "mysql", Version: "8.0", Available: true, Arch: "all", URL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.39.tar.gz"},
		{Type: "mysql", Version: "5.7", Available: true, Arch: "all", URL: "https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44.tar.gz"},

		// Nginx (source - requires compilation)
		{Type: "nginx", Version: "1.25", Available: true, Arch: "all", URL: "https://nginx.org/download/nginx-1.25.5.tar.gz"},
		{Type: "nginx", Version: "1.24", Available: true, Arch: "all", URL: "https://nginx.org/download/nginx-1.24.0.tar.gz"},

		// Apache (source - requires compilation)
		{Type: "apache", Version: "2.4", Available: true, Arch: "all", URL: "https://archive.apache.org/httpd/httpd-2.4.62.tar.gz"},

		// Redis (source - compiles quickly)
		{Type: "redis", Version: "7.2", Available: true, Arch: "all", URL: "https://github.com/redis/redis/archive/refs/tags/7.2.7.tar.gz"},
		{Type: "redis", Version: "7.0", Available: true, Arch: "all", URL: "https://github.com/redis/redis/archive/refs/tags/7.0.15.tar.gz"},
	}

	sm := &ServiceManager{
		services:      make(map[string]*Service),
		available:     availableVersions,
		baseDir:       baseDir,
		installStatus: make(map[string]int),
	}

	sm.loadInstalledServices()
	return sm
}

func (sm *ServiceManager) GetAvailableVersions(svcType string) []ServiceVersion {
	var result []ServiceVersion
	arch := runtime.GOARCH

	for _, v := range sm.available {
		if svcType == "" || v.Type == svcType {
			if v.Arch == "all" || v.Arch == arch {
				result = append(result, v)
			}
		}
	}
	return result
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

	var err error
	switch svcType {
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

			sm.updateInstallProgress("mariadb", version, 70)
			return sm.compileMariaDB(version, installDir, configDir, dataDir)
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
		return fmt.Errorf("mariadb make failed: %w", err)
	}

	installCmd := exec.Command("make", "install")
	installCmd.Dir = buildDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
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
	sourceDir := filepath.Join(installDir, fmt.Sprintf("nginx-%s", version))
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory not found: %s", sourceDir)
	}

	pcreURL := "https://downloads.sourceforge.net/project/pcre/pcre/8.45/pcre-8.45.tar.gz"
	opensslURL := "https://www.openssl.org/source/openssl-3.1.4.tar.gz"
	zlibURL := "https://zlib.net/zlib-1.3.tar.gz"

	depsDir := filepath.Join(installDir, "deps")
	os.MkdirAll(depsDir, 0755)

	sm.updateInstallProgress("nginx", version, 72)
	fmt.Println("Downloading dependencies...")

	sm.downloadAndExtract(pcreURL, depsDir, nil)
	sm.downloadAndExtract(opensslURL, depsDir, nil)
	sm.downloadAndExtract(zlibURL, depsDir, nil)

	sm.updateInstallProgress("nginx", version, 80)
	fmt.Println("Compiling Nginx...")

	binDir := filepath.Join(installDir, "nginx-bin")
	os.MkdirAll(binDir, 0755)

	pcreDir := filepath.Join(depsDir, "pcre-8.45")
	opensslDir := filepath.Join(depsDir, "openssl-3.1.4")
	zlibDir := filepath.Join(depsDir, "zlib-1.3")

	configureCmd := exec.Command("./configure",
		"--prefix="+binDir,
		"--with-pcre="+pcreDir,
		"--with-openssl="+opensslDir,
		"--with-zlib="+zlibDir,
		"--with-http_ssl_module",
		"--with-http_v2_module",
		"--with-http_realip_module",
		"--with-http_gzip_static_module",
	)
	configureCmd.Dir = sourceDir
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		return fmt.Errorf("nginx configure failed: %w", err)
	}

	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = sourceDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("nginx make failed: %w", err)
	}

	installCmd := exec.Command("make", "install")
	installCmd.Dir = sourceDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("nginx install failed: %w", err)
	}

	sm.createNginxConfig(configDir)
	sm.updateInstallProgress("nginx", version, 100)
	fmt.Printf("✅ Nginx %s installed to %s\n", version, binDir)
	return nil
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
	sourceDir := filepath.Join(installDir, fmt.Sprintf("httpd-%s", version))
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory not found: %s", sourceDir)
	}

	aprURL := "https://archive.apache.org/dist/apr/apr-1.7.4.tar.gz"
	aprUtilURL := "https://archive.apache.org/dist/apr/apr-util-1.6.3.tar.gz"

	depsDir := filepath.Join(installDir, "deps")
	os.MkdirAll(depsDir, 0755)

	sm.updateInstallProgress("apache", version, 72)
	fmt.Println("Downloading APR dependencies...")

	sm.downloadAndExtract(aprURL, depsDir, nil)
	sm.downloadAndExtract(aprUtilURL, depsDir, nil)

	binDir := filepath.Join(installDir, "apache-bin")
	os.MkdirAll(binDir, 0755)

	aprDir := filepath.Join(depsDir, "apr-1.7.4")
	aprUtilDir := filepath.Join(depsDir, "apr-util-1.6.3")

	sm.updateInstallProgress("apache", version, 75)

	for _, dep := range []struct{ dir, name string }{
		{aprDir, "APR"},
		{aprUtilDir, "APR-Util"},
	} {
		configureCmd := exec.Command("./configure", "--prefix="+filepath.Join(depsDir, dep.name))
		configureCmd.Dir = dep.dir
		configureCmd.Stdout = os.Stdout
		configureCmd.Stderr = os.Stderr
		configureCmd.Run()

		makeCmd := exec.Command("make", "-j4")
		makeCmd.Dir = dep.dir
		makeCmd.Stdout = os.Stdout
		makeCmd.Stderr = os.Stderr
		makeCmd.Run()

		installCmd := exec.Command("make", "install")
		installCmd.Dir = dep.dir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		installCmd.Run()
	}

	sm.updateInstallProgress("apache", version, 85)
	fmt.Println("Compiling Apache...")

	configureCmd := exec.Command("./configure",
		"--prefix="+binDir,
		"--enable-so",
		"--enable-ssl",
		"--enable-mods-shared=most",
		"--with-apr="+filepath.Join(depsDir, "APR"),
		"--with-apr-util="+filepath.Join(depsDir, "APR"),
		"--enable-mpms-shared=all",
		"--with-mpm=event",
	)
	configureCmd.Dir = sourceDir
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		return fmt.Errorf("apache configure failed: %w", err)
	}

	makeCmd := exec.Command("make", "-j4")
	makeCmd.Dir = sourceDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("apache make failed: %w", err)
	}

	installCmd := exec.Command("make", "install")
	installCmd.Dir = sourceDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("apache install failed: %w", err)
	}

	sm.createApacheConfig(configDir, dataDir, binDir)
	sm.updateInstallProgress("apache", version, 100)
	fmt.Printf("✅ Apache %s installed to %s\n", version, binDir)
	return nil
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
	sourceDir := filepath.Join(installDir, fmt.Sprintf("redis-%s.%s", version, version))
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		sourceDir = filepath.Join(installDir, "redis-7.2.7")
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			sourceDir = filepath.Join(installDir, "redis-7.0.15")
		}
	}

	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory not found: %s", sourceDir)
	}

	binDir := filepath.Join(installDir, "redis-bin")
	os.MkdirAll(binDir, 0755)

	fmt.Println("Compiling Redis...")

	makeCmd := exec.Command("make", "-j4", "PREFIX="+binDir)
	makeCmd.Dir = sourceDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("redis make failed: %w", err)
	}

	installCmd := exec.Command("make", "install", "PREFIX="+binDir)
	installCmd.Dir = sourceDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("redis install failed: %w", err)
	}

	sm.createRedisConfig(configDir, dataDir)

	os.MkdirAll(dataDir, 0755)

	fmt.Println("Initializing Redis...")
	os.WriteFile(filepath.Join(dataDir, ".initialized"), []byte(time.Now().Format(time.RFC3339)), 0644)

	sm.updateInstallProgress("redis", version, 100)
	fmt.Printf("✅ Redis %s installed to %s\n", version, binDir)
	return nil
}

func (sm *ServiceManager) UninstallService(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	svc, ok := sm.services[name]
	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	sm.StopService(name)

	installDir := svc.BinaryDir
	configDir := svc.ConfigDir
	dataDir := svc.DataDir

	os.RemoveAll(installDir)
	os.RemoveAll(configDir)
	os.RemoveAll(dataDir)

	delete(sm.services, name)
	return nil
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
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if progressCallback != nil && resp.ContentLength > 0 {
		pr := &progressReader{
			Reader: resp.Body,
			Total:  resp.ContentLength,
			OnProg: progressCallback,
		}
		reader = pr
	}

	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
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

		target := filepath.Join(targetDir, header.Name)

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
	Reader  io.Reader
	Total   int64
	Current int64
	OnProg  func(int)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 && pr.OnProg != nil {
		pr.OnProg(int(float64(pr.Current) / float64(pr.Total) * 100))
	}
	return
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

	var cmd *exec.Cmd
	var binaryPath string

	switch svc.Type {
	case "mariadb":
		binaryPath = sm.findMariaDBBinary(svc.BinaryDir)
		if binaryPath == "" {
			return fmt.Errorf("MariaDB binary not found")
		}
		cmd = sm.startMariaDB(svc, binaryPath)
	case "nginx":
		binaryPath = filepath.Join(svc.BinaryDir, "nginx-bin", "sbin", "nginx")
		cmd = sm.startNginx(svc, binaryPath)
	case "apache":
		binaryPath = filepath.Join(svc.BinaryDir, "apache-bin", "bin", "httpd")
		cmd = sm.startApache(svc, binaryPath)
	case "redis":
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

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	sm.mu.Lock()
	svc.Status = "running"
	svc.PID = cmd.Process.Pid
	sm.mu.Unlock()

	sm.saveServiceStatus(svc)
	sm.savePID(name, cmd.Process.Pid)

	fmt.Printf("✅ Service %s started (PID: %d)\n", name, cmd.Process.Pid)
	return nil
}

func (sm *ServiceManager) StopService(name string) error {
	sm.mu.Lock()
	svc, ok := sm.services[name]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if svc.Status != "running" {
		return fmt.Errorf("service %s is not running", name)
	}

	pid := svc.PID
	if pid == 0 {
		pid = sm.loadPID(name)
	}

	if pid > 0 {
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("process not found: %w", err)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			process.Signal(syscall.SIGKILL)
		}

		time.Sleep(500 * time.Millisecond)
	}

	sm.mu.Lock()
	svc.Status = "stopped"
	svc.PID = 0
	sm.mu.Unlock()

	sm.saveServiceStatus(svc)
	os.Remove(sm.getPIDFile(name))

	fmt.Printf("⏹️ Service %s stopped\n", name)
	return nil
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

func (sm *ServiceManager) GetServices() []*Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var services []*Service
	for _, service := range sm.services {
		services = append(services, service)
	}
	return services
}

func (sm *ServiceManager) GetService(name string) *Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.services[name]
}

func (sm *ServiceManager) FormatStatus() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.services) == 0 {
		return "No services configured"
	}

	var status strings.Builder
	status.WriteString("Services:\n")

	for _, service := range sm.services {
		icon := "[ ]"
		if service.Status == "running" {
			icon = "[v]"
		}

		status.WriteString(fmt.Sprintf("%s %s (%s) - localhost:%d\n",
			icon, service.Name, service.Type, service.Port))
	}

	return status.String()
}

func (sm *ServiceManager) StartAll() error {
	sm.mu.RLock()
	servicesToStart := make([]*Service, 0, len(sm.services))
	for _, service := range sm.services {
		if service.Status == "stopped" {
			servicesToStart = append(servicesToStart, service)
		}
	}
	sm.mu.RUnlock()

	for _, service := range servicesToStart {
		if err := sm.StartService(service.Name); err != nil {
			fmt.Printf("❌ Failed to start %s: %v\n", service.Name, err)
		}
	}

	return nil
}

func (sm *ServiceManager) StopAll() error {
	sm.mu.RLock()
	servicesToStop := make([]*Service, 0, len(sm.services))
	for _, service := range sm.services {
		if service.Status == "running" {
			servicesToStop = append(servicesToStop, service)
		}
	}
	sm.mu.RUnlock()

	for _, service := range servicesToStop {
		if err := sm.StopService(service.Name); err != nil {
			fmt.Printf("❌ Failed to stop %s: %v\n", service.Name, err)
		}
	}

	return nil
}

func (sm *ServiceManager) saveServiceStatus(svc *Service) {
	statusPath := filepath.Join(sm.baseDir, "services.json")
	var services []*Service

	if data, err := os.ReadFile(statusPath); err == nil {
		json.Unmarshal(data, &services)
	}

	var found bool
	for i, s := range services {
		if s.Name == svc.Name {
			services[i] = svc
			found = true
			break
		}
	}

	if !found {
		services = append(services, svc)
	}

	data, _ := json.MarshalIndent(services, "", "  ")
	os.WriteFile(statusPath, data, 0644)
}

func (sm *ServiceManager) loadInstalledServices() {
	statusPath := filepath.Join(sm.baseDir, "services.json")
	if data, err := os.ReadFile(statusPath); err == nil {
		var services []*Service
		json.Unmarshal(data, &services)
		for _, svc := range services {
			sm.services[svc.Name] = svc
		}
	}
}

func (sm *ServiceManager) savePID(name string, pid int) error {
	pidFile := sm.getPIDFile(name)
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
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

func (sm *ServiceManager) createNginxConfig(configDir string) error {
	os.MkdirAll(filepath.Join(configDir, "sites-enabled"), 0755)

	nginxConf := fmt.Sprintf(`worker_processes  auto;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;

    access_log  logs/access.log;
    error_log   logs/error.log;

    server {
        listen       80;
        server_name  localhost;

        location / {
            root   html;
            index  index.html index.htm;
        }
    }

    include %s/sites-enabled/*.conf;
}
`, configDir)

	configPath := filepath.Join(configDir, "nginx.conf")
	return os.WriteFile(configPath, []byte(nginxConf), 0644)
}

func (sm *ServiceManager) createApacheConfig(configDir, dataDir, installDir string) error {
	os.MkdirAll(filepath.Join(configDir, "sites-available"), 0755)
	os.MkdirAll(filepath.Join(configDir, "sites-enabled"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "htdocs"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "logs"), 0755)

	httpdConf := fmt.Sprintf(`ServerRoot %s
Listen 8080

LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule authz_host_module modules/mod_authz_host.so
LoadModule log_config_module modules/mod_log_config.so
LoadModule dir_module modules/mod_dir.so
LoadModule mime_module modules/mod_mime.so
LoadModule rewrite_module modules/mod_rewrite.so

DocumentRoot "%s/htdocs"
<Directory "%s/htdocs">
    Options Indexes FollowSymLinks
    AllowOverride All
    Require all granted
</Directory>

ErrorLog "%s/logs/error_log"
CustomLog "%s/logs/access_log" combined

Include %s/sites-enabled/*.conf
`, installDir, dataDir, dataDir, dataDir, dataDir, configDir)

	configPath := filepath.Join(configDir, "httpd.conf")
	return os.WriteFile(configPath, []byte(httpdConf), 0644)
}

func (sm *ServiceManager) createRedisConfig(configDir, dataDir string) error {
	redisConf := fmt.Sprintf(`port 6379
daemonize no
dir %s
logfile %s/redis.log
dbfilename dump.rdb
save 900 1
save 300 10
`, dataDir, dataDir)

	configPath := filepath.Join(configDir, "redis.conf")
	return os.WriteFile(configPath, []byte(redisConf), 0644)
}
