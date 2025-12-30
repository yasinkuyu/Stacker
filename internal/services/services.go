package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Service struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // mysql, postgres, redis, meilisearch, minio, typesense
	Port      int    `json:"port"`
	Status    string `json:"status"` // running, stopped
	Version   string `json:"version"`
	DataDir   string `json:"data_dir"`
	ConfigDir string `json:"config_dir"`
}

type ServiceManager struct {
	services map[string]*Service
	mu       sync.RWMutex
	baseDir  string
}

func NewServiceManager() *ServiceManager {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".stacker-app", "services")
	os.MkdirAll(baseDir, 0755)

	return &ServiceManager{
		services: make(map[string]*Service),
		baseDir:  baseDir,
	}
}

func (sm *ServiceManager) AddService(service *Service) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	dataDir := filepath.Join(sm.baseDir, service.Name, "data")
	configDir := filepath.Join(sm.baseDir, service.Name, "config")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)

	service.DataDir = dataDir
	service.ConfigDir = configDir
	service.Status = "stopped"

	sm.services[service.Name] = service
	return nil
}

func (sm *ServiceManager) StartService(name string) error {
	sm.mu.Lock()
	service, ok := sm.services[name]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if service.Status == "running" {
		return fmt.Errorf("service %s is already running", name)
	}

	var cmd *exec.Cmd

	switch service.Type {
	case "mysql":
		cmd = sm.startMySQL(service)
	case "postgres":
		cmd = sm.startPostgreSQL(service)
	case "redis":
		cmd = sm.startRedis(service)
	case "meilisearch":
		cmd = sm.startMeiliSearch(service)
	case "minio":
		cmd = sm.startMinIO(service)
	default:
		return fmt.Errorf("unsupported service type: %s", service.Type)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	sm.mu.Lock()
	service.Status = "running"
	sm.mu.Unlock()

	return nil
}

func (sm *ServiceManager) StopService(name string) error {
	sm.mu.Lock()
	service, ok := sm.services[name]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if service.Status != "running" {
		return fmt.Errorf("service %s is not running", name)
	}

	// Process kill işlemi
	sm.mu.Lock()
	service.Status = "stopped"
	sm.mu.Unlock()

	return nil
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

func (sm *ServiceManager) RemoveService(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.services[name]; !ok {
		return fmt.Errorf("service %s not found", name)
	}

	delete(sm.services, name)
	return nil
}

func (sm *ServiceManager) StopAll() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for name, service := range sm.services {
		if service.Status == "running" {
			service.Status = "stopped"
			fmt.Printf("⏹️  Stopped %s\n", name)
		}
	}
	return nil
}

func (sm *ServiceManager) startMySQL(service *Service) *exec.Cmd {
	cmd := exec.Command("mysqld",
		"--datadir="+service.DataDir,
		"--port="+fmt.Sprintf("%d", service.Port),
		fmt.Sprintf("--socket=%s/mysql.sock", service.DataDir),
	)
	return cmd
}

func (sm *ServiceManager) startPostgreSQL(service *Service) *exec.Cmd {
	dataDir := filepath.Join(service.DataDir, "pgdata")
	cmd := exec.Command("pg_ctl",
		"-D", dataDir,
		"-l", filepath.Join(dataDir, "logfile"),
		"start",
	)
	return cmd
}

func (sm *ServiceManager) startRedis(service *Service) *exec.Cmd {
	cmd := exec.Command("redis-server",
		"--port", fmt.Sprintf("%d", service.Port),
		"--dir", service.DataDir,
	)
	return cmd
}

func (sm *ServiceManager) startMeiliSearch(service *Service) *exec.Cmd {
	cmd := exec.Command("meilisearch",
		"--http-port", fmt.Sprintf("%d", service.Port),
		"--db-path", filepath.Join(service.DataDir, "meili"),
	)
	return cmd
}

func (sm *ServiceManager) startMinIO(service *Service) *exec.Cmd {
	cmd := exec.Command("minio",
		"server",
		service.DataDir,
		"--address", fmt.Sprintf(":%d", service.Port),
	)
	return cmd
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
		icon := "⏹️"
		if service.Status == "running" {
			icon = "✅"
		}

		status.WriteString(fmt.Sprintf("%s %s (%s) - %s:%d\n",
			icon, service.Name, service.Type, "localhost", service.Port))
	}

	return status.String()
}
