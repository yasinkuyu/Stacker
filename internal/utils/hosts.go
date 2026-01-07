package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const stackerMarker = "# stacker-app"

// HostEntry represents a single entry in the hosts file
type HostEntry struct {
	IP        string   `json:"ip"`
	Hostname  string   `json:"hostname"`
	Aliases   []string `json:"aliases,omitempty"`
	Comment   string   `json:"comment,omitempty"`
	Enabled   bool     `json:"enabled"`
	Group     string   `json:"group"` // "stacker", "custom", "system"
	LineIndex int      `json:"lineIndex"`
	RawLine   string   `json:"-"`
}

// HostsManager provides advanced hosts file management
type HostsManager struct {
	hostsPath string
	backupDir string
}

// NewHostsManager creates a new hosts manager
func NewHostsManager() *HostsManager {
	stackerDir := GetStackerDir()
	return &HostsManager{
		hostsPath: GetHostsPath(),
		backupDir: filepath.Join(stackerDir, "backups", "hosts"),
	}
}

// GetHostsPath returns the platform-specific hosts file path
func GetHostsPath() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\Windows\System32\drivers\etc\hosts`
	case "darwin", "linux":
		return "/etc/hosts"
	default:
		return "/etc/hosts"
	}
}

// GetAllEntries parses and returns all entries from the hosts file
func (hm *HostsManager) GetAllEntries() ([]HostEntry, error) {
	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read hosts file: %w", err)
	}

	var entries []HostEntry
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineIndex := 0

	// Regex to parse hosts entries
	// Matches: IP<whitespace>hostname [alias1 alias2...] [# comment]
	entryRegex := regexp.MustCompile(`^(#?\s*)(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}|[a-fA-F0-9:]+)\s+(.+)$`)

	for scanner.Scan() {
		line := scanner.Text()
		lineIndex++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip pure comment lines (not disabled entries)
		if strings.HasPrefix(strings.TrimSpace(line), "#") && !entryRegex.MatchString(line) {
			continue
		}

		matches := entryRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		prefix := matches[1]
		ip := matches[2]
		rest := matches[3]

		// Determine if entry is disabled (commented out)
		enabled := !strings.HasPrefix(strings.TrimSpace(prefix), "#")

		// Parse hostname, aliases, and comment
		var hostname string
		var aliases []string
		var comment string

		// Split by # to separate hostname/aliases from comment
		parts := strings.SplitN(rest, "#", 2)
		hostPart := strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			comment = strings.TrimSpace(parts[1])
		}

		// Split hostnames
		hostnames := strings.Fields(hostPart)
		if len(hostnames) > 0 {
			hostname = hostnames[0]
			if len(hostnames) > 1 {
				aliases = hostnames[1:]
			}
		}

		// Determine group
		group := "system"
		if strings.Contains(comment, "stacker") {
			group = "stacker"
		} else if !isSystemEntry(hostname, ip) {
			group = "custom"
		}

		entries = append(entries, HostEntry{
			IP:        ip,
			Hostname:  hostname,
			Aliases:   aliases,
			Comment:   comment,
			Enabled:   enabled,
			Group:     group,
			LineIndex: lineIndex,
			RawLine:   line,
		})
	}

	return entries, nil
}

// isSystemEntry checks if an entry is a typical system entry
func isSystemEntry(hostname, ip string) bool {
	systemHosts := map[string]bool{
		"localhost":             true,
		"localhost.localdomain": true,
		"local":                 true,
		"broadcasthost":         true,
		"ip6-localhost":         true,
		"ip6-loopback":          true,
		"ip6-localnet":          true,
		"ip6-mcastprefix":       true,
		"ip6-allnodes":          true,
		"ip6-allrouters":        true,
	}
	return systemHosts[hostname]
}

// AddEntry adds a new entry to the hosts file
func (hm *HostsManager) AddEntry(entry HostEntry) error {
	// Create backup first
	if _, err := hm.CreateBackup(); err != nil {
		LogError(fmt.Sprintf("Failed to create backup before adding entry: %v", err))
	}

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	// Check for duplicates
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, entry.Hostname) && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			return fmt.Errorf("hostname %s already exists", entry.Hostname)
		}
	}

	// Build entry line
	line := hm.buildEntryLine(entry)

	// Append to file
	f, err := os.OpenFile(hm.hostsPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open hosts file: %w (try running with sudo)", err)
	}
	defer f.Close()

	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w", err)
	}

	return nil
}

// UpdateEntry updates an existing entry by line index
func (hm *HostsManager) UpdateEntry(lineIndex int, entry HostEntry) error {
	// Create backup first
	if _, err := hm.CreateBackup(); err != nil {
		LogError(fmt.Sprintf("Failed to create backup before updating entry: %v", err))
	}

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if lineIndex < 1 || lineIndex > len(lines) {
		return fmt.Errorf("invalid line index: %d", lineIndex)
	}

	lines[lineIndex-1] = hm.buildEntryLine(entry)

	return hm.writeLines(lines)
}

// DeleteEntry removes an entry by line index
func (hm *HostsManager) DeleteEntry(lineIndex int) error {
	// Create backup first
	if _, err := hm.CreateBackup(); err != nil {
		LogError(fmt.Sprintf("Failed to create backup before deleting entry: %v", err))
	}

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if lineIndex < 1 || lineIndex > len(lines) {
		return fmt.Errorf("invalid line index: %d", lineIndex)
	}

	// Remove the line
	newLines := append(lines[:lineIndex-1], lines[lineIndex:]...)

	return hm.writeLines(newLines)
}

// ToggleEntry enables or disables an entry by commenting/uncommenting
func (hm *HostsManager) ToggleEntry(lineIndex int) error {
	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if lineIndex < 1 || lineIndex > len(lines) {
		return fmt.Errorf("invalid line index: %d", lineIndex)
	}

	line := lines[lineIndex-1]
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "#") {
		// Uncomment - remove leading #
		lines[lineIndex-1] = strings.TrimPrefix(trimmed, "#")
		lines[lineIndex-1] = strings.TrimSpace(lines[lineIndex-1])
	} else {
		// Comment - add leading #
		lines[lineIndex-1] = "# " + line
	}

	return hm.writeLines(lines)
}

// CreateBackup creates a backup of the current hosts file
func (hm *HostsManager) CreateBackup() (string, error) {
	// Ensure backup directory exists
	if err := os.MkdirAll(hm.backupDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create backup directory: %w", err)
	}

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return "", fmt.Errorf("cannot read hosts file: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(hm.backupDir, fmt.Sprintf("hosts_%s.bak", timestamp))

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("cannot write backup file: %w", err)
	}

	return backupPath, nil
}

// GetBackups returns a list of available backups
func (hm *HostsManager) GetBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(hm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("cannot read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(hm.backupDir, entry.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	return backups, nil
}

// BackupInfo represents a backup file
type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// RestoreBackup restores the hosts file from a backup
func (hm *HostsManager) RestoreBackup(backupPath string) error {
	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	}

	// Create a backup of current state before restoring
	if _, err := hm.CreateBackup(); err != nil {
		LogError(fmt.Sprintf("Failed to create backup before restore: %v", err))
	}

	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("cannot read backup file: %w", err)
	}

	if err := os.WriteFile(hm.hostsPath, content, 0644); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w (try running with sudo)", err)
	}

	return nil
}

// ExportHosts returns the current hosts file content as a string
func (hm *HostsManager) ExportHosts() (string, error) {
	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return "", fmt.Errorf("cannot read hosts file: %w", err)
	}
	return string(content), nil
}

// ImportHosts replaces the hosts file with the provided content
func (hm *HostsManager) ImportHosts(content string) error {
	// Create backup first
	if _, err := hm.CreateBackup(); err != nil {
		LogError(fmt.Sprintf("Failed to create backup before import: %v", err))
	}

	if err := os.WriteFile(hm.hostsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w (try running with sudo)", err)
	}

	return nil
}

// AddToHosts adds a hostname to the hosts file (legacy compatibility)
func AddToHosts(hostname string) error {
	hm := NewHostsManager()
	return hm.AddEntry(HostEntry{
		IP:       "127.0.0.1",
		Hostname: hostname,
		Comment:  stackerMarker,
		Enabled:  true,
		Group:    "stacker",
	})
}

// RemoveFromHosts removes a hostname from the hosts file (legacy compatibility)
func RemoveFromHosts(hostname string) error {
	hm := NewHostsManager()
	entries, err := hm.GetAllEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Hostname == hostname {
			return hm.DeleteEntry(entry.LineIndex)
		}
	}

	return fmt.Errorf("hostname %s not found", hostname)
}

// buildEntryLine constructs a hosts file line from an entry
func (hm *HostsManager) buildEntryLine(entry HostEntry) string {
	var parts []string
	parts = append(parts, entry.IP)
	parts = append(parts, entry.Hostname)

	if len(entry.Aliases) > 0 {
		parts = append(parts, entry.Aliases...)
	}

	line := strings.Join(parts, "\t")

	if entry.Comment != "" {
		line += " # " + entry.Comment
	} else if entry.Group == "stacker" {
		line += " " + stackerMarker
	}

	if !entry.Enabled {
		line = "# " + line
	}

	return line
}

// writeLines writes a slice of lines to the hosts file
func (hm *HostsManager) writeLines(lines []string) error {
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(hm.hostsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write to hosts file: %w (try running with sudo)", err)
	}
	return nil
}

// FlushDNS flushes the DNS cache (platform-specific)
func FlushDNS() error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "dscacheutil"
		args = []string{"-flushcache"}
	case "linux":
		// Try systemd-resolve first, then nscd
		cmd = "systemd-resolve"
		args = []string{"--flush-caches"}
	case "windows":
		cmd = "ipconfig"
		args = []string{"/flushdns"}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	LogService("dns", "flush", fmt.Sprintf("Running: %s %v", cmd, args))
	return nil
}
