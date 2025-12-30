package node

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type NodeVersion struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Default bool   `json:"default"`
}

type NodeManager struct {
	nvmPath string
	baseDir string
}

func NewNodeManager() *NodeManager {
	home, _ := os.UserHomeDir()
	return &NodeManager{
		nvmPath: filepath.Join(home, ".nvm"),
		baseDir: filepath.Join(home, ".stacker-app", "node"),
	}
}

func (nm *NodeManager) GetVersions() ([]NodeVersion, error) {
	var versions []NodeVersion

	if nm.NVMInstalled() {
		versions = nm.GetNVMVersions()
	}

	return versions, nil
}

func (nm *NodeManager) NVMInstalled() bool {
	_, err := exec.LookPath("nvm")
	return err == nil || nm.HasNVM()
}

func (nm *NodeManager) HasNVM() bool {
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".nvm"))
	return err == nil
}

func (nm *NodeManager) GetNVMVersions() []NodeVersion {
	var versions []NodeVersion

	versionsDir := filepath.Join(nm.nvmPath, "versions", "node")
	if dirs, err := os.ReadDir(versionsDir); err == nil {
		for _, dir := range dirs {
			versions = append(versions, NodeVersion{
				Version: dir.Name(),
				Path:    filepath.Join(versionsDir, dir.Name()),
				Default: false,
			})
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	defaultVersion := nm.GetCurrentNVMVersion()
	for i := range versions {
		if versions[i].Version == defaultVersion {
			versions[i].Default = true
		}
	}

	return versions
}

func (nm *NodeManager) getCurrentNVMVersion() string {
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (nm *NodeManager) GetCurrentNVMVersion() string {
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (nm *NodeManager) SetDefault(version string) error {
	if nm.NVMInstalled() {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("source ~/.nvm/nvm.sh && nvm alias default %s", version))
		return cmd.Run()
	}
	return fmt.Errorf("nvm not installed")
}

func (nm *NodeManager) InstallVersion(version string) error {
	if nm.NVMInstalled() {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("source ~/.nvm/nvm.sh && nvm install %s", version))
		return cmd.Run()
	}
	return fmt.Errorf("nvm not installed")
}

func (nm *NodeManager) UseVersion(version string) error {
	if nm.NVMInstalled() {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("source ~/.nvm/nvm.sh && nvm use %s", version))
		return cmd.Run()
	}
	return fmt.Errorf("nvm not installed")
}

func (nm *NodeManager) GetVersionForSite(sitePath string) string {
	nvmrcFile := filepath.Join(sitePath, ".nvmrc")
	if data, err := os.ReadFile(nvmrcFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (nm *NodeManager) FormatVersions() string {
	versions, _ := nm.GetVersions()

	if len(versions) == 0 {
		return "No Node.js versions detected. Install nvm to manage Node versions."
	}

	var output strings.Builder
	output.WriteString("Node.js versions:\n")

	for _, version := range versions {
		prefix := "  "
		if version.Default {
			prefix = "* "
		}
		output.WriteString(fmt.Sprintf("%s%s\n", prefix, version.Version))
	}

	return output.String()
}

func (nm *NodeManager) ExecuteCommand(version string, args ...string) error {
	var cmd *exec.Cmd

	if nm.NVMInstalled() {
		cmd = exec.Command("bash", "-c", fmt.Sprintf("source ~/.nvm/nvm.sh && nvm use %s && node %s", version, strings.Join(args, " ")))
	} else {
		cmd = exec.Command("node", args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
