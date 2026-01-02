package ssl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// EnsureMkcert downloads and verifies mkcert binary if not present
func EnsureMkcert(stackerDir string) (string, error) {
	mkcertDir := filepath.Join(stackerDir, "bin", "mkcert")
	if err := os.MkdirAll(mkcertDir, 0755); err != nil {
		return "", err
	}

	mkcertPath := filepath.Join(mkcertDir, "mkcert")
	if runtime.GOOS == "windows" {
		mkcertPath += ".exe"
	}

	// Check if already exists
	if _, err := os.Stat(mkcertPath); err == nil {
		return mkcertPath, nil
	}

	// Download mkcert
	fmt.Println("📥 Downloading mkcert...")
	version := "v1.4.4"
	var downloadURL string

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			downloadURL = fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-darwin-arm64", version, version)
		} else {
			downloadURL = fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-darwin-amd64", version, version)
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			downloadURL = fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-linux-arm64", version, version)
		} else {
			downloadURL = fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-linux-amd64", version, version)
		}
	case "windows":
		downloadURL = fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-windows-amd64.exe", version, version)
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download mkcert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download mkcert: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(mkcertPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	// Make executable
	if err := os.Chmod(mkcertPath, 0755); err != nil {
		return "", err
	}

	fmt.Println("✅ mkcert downloaded successfully")
	return mkcertPath, nil
}

// InstallRootCA installs mkcert root CA to system trust store
func InstallRootCA(mkcertPath string) error {
	// Check if already installed
	cmd := exec.Command(mkcertPath, "-CAROOT")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check CAROOT: %w", err)
	}

	caRoot := string(output)
	caFile := filepath.Join(caRoot, "rootCA.pem")

	// If CA already exists, assume it's installed
	if _, err := os.Stat(caFile); err == nil {
		fmt.Println("✅ mkcert root CA already installed")
		return nil
	}

	fmt.Println("🔐 Installing mkcert root CA to system trust store...")
	fmt.Println("⚠️  You may be prompted for your password")

	cmd = exec.Command(mkcertPath, "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install root CA: %w", err)
	}

	fmt.Println("✅ Root CA installed successfully")
	return nil
}

// GenerateCertificate creates SSL certificate for a domain
func GenerateCertificate(mkcertPath, stackerDir, domain string) (certPath, keyPath string, err error) {
	certsDir := filepath.Join(stackerDir, "certs", domain)
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return "", "", err
	}

	certFile := filepath.Join(certsDir, "cert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	// Check if certificate already exists
	if _, err := os.Stat(certFile); err == nil {
		return certFile, keyFile, nil
	}

	fmt.Printf("🔐 Generating SSL certificate for %s...\n", domain)

	// Run mkcert to generate certificate
	cmd := exec.Command(mkcertPath, "-cert-file", certFile, "-key-file", keyFile, domain)
	cmd.Dir = certsDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate certificate: %w", err)
	}

	fmt.Printf("✅ SSL certificate generated for %s\n", domain)
	return certFile, keyFile, nil
}
