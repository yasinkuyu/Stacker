package server

import (
	"crypto/tls"
	"fmt"
	"stacker-app/internal/config"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Server struct {
	cfg    *config.Config
	server *http.Server
	fpmMgr *PHPFPMManager
}

type PHPFPMManager struct {
	processes map[string]*exec.Cmd
	mu        sync.Mutex
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:    cfg,
		fpmMgr: NewPHPFPMManager(),
	}
}

func NewPHPFPMManager() *PHPFPMManager {
	return &PHPFPMManager{
		processes: make(map[string]*exec.Cmd),
	}
}

func (p *PHPFPMManager) Start(siteName, sitePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.processes[siteName]; exists {
		return nil
	}

	routerFile := filepath.Join(sitePath, "public", "index.php")
	if _, err := os.Stat(routerFile); os.IsNotExist(err) {
		return fmt.Errorf("index.php not found at %s", routerFile)
	}

	fcgiAddr := fmt.Sprintf("127.0.0.1:%d", 9000+len(p.processes))

	cmd := exec.Command("php-fpm")
	cmd.Env = append(os.Environ(),
		"PHP_FCGI_CHILDREN=1",
		fmt.Sprintf("PHP_FCGI_LISTEN_ADDRESS=%s", fcgiAddr),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start php-fpm: %w", err)
	}

	p.processes[siteName] = cmd
	return nil
}

func (p *PHPFPMManager) Stop(siteName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cmd, exists := p.processes[siteName]; exists {
		cmd.Process.Kill()
		delete(p.processes, siteName)
	}
}

func (p *PHPFPMManager) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, cmd := range p.processes {
		cmd.Process.Kill()
		delete(p.processes, name)
	}
}

func (s *Server) Start() error {
	certPath, keyPath, err := s.generateCertificates()
	if err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{},
		MinVersion:   tls.VersionTLS12,
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load certificates: %w", err)
	}
	tlsConfig.Certificates = append(tlsConfig.Certificates, cert)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		site := s.cfg.GetSite(host)

		if site == nil {
			s.handleNotFound(w, r)
			return
		}

		s.handlePHPRequest(w, r, site)
	})

	s.server = &http.Server{
		Addr:      fmt.Sprintf(":%d", s.cfg.Port),
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(listener, tlsConfig)
	log.Printf("Server listening on %s", s.server.Addr)
	return s.server.Serve(tlsListener)
}

func (s *Server) Stop() {
	if s.server != nil {
		s.server.Close()
	}
	s.fpmMgr.StopAll()
}

func (s *Server) handlePHPRequest(w http.ResponseWriter, r *http.Request, site *config.Site) {
	docRoot := filepath.Join(site.Path, "public")
	staticFile := filepath.Join(docRoot, r.URL.Path)

	if r.URL.Path != "/" {
		if _, err := os.Stat(staticFile); err == nil {
			http.ServeFile(w, r, staticFile)
			return
		}
	}

	routerFile := filepath.Join(docRoot, "index.php")

	targetURL, _ := url.Parse("fcgi://127.0.0.1:9000")
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/index.php"
		req.Host = "localhost"
		req.Header.Set("SCRIPT_FILENAME", routerFile)
		req.Header.Set("DOCUMENT_ROOT", docRoot)
		req.Header.Set("REQUEST_URI", r.RequestURI)
	}

	proxy.ServeHTTP(w, r)
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Site not found"))
}

func (s *Server) generateCertificates() (string, string, error) {
	home, _ := os.UserHomeDir()
	certDir := filepath.Join(home, ".stacker-app", "certs")
	os.MkdirAll(certDir, 0755)

	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	if _, err := os.Stat(certPath); err == nil {
		return certPath, keyPath, nil
	}

	cmd := exec.Command("mkcert", "-install")
	cmd.Run()

	cmd = exec.Command("mkcert", "*.test", "localhost")
	cmd.Dir = certDir
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("mkcert failed: %w", err)
	}

	return certPath, keyPath, nil
}
