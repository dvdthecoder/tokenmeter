// Package claudecode configures Claude Code CLI to route through tokenmeter.
package claudecode

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/backends"
)

//go:embed skills
var skillsFS embed.FS

// skillNames are the files we own in ~/.claude/skills/ — used for clean uninstall.
var skillNames = []string{"proxy-status.md", "proxy-report.md", "proxy-purge.md"}

func init() {
	backends.Register(&Adapter{})
}

// Adapter configures Claude Code CLI.
type Adapter struct{}

func (a *Adapter) Name() string { return "claudecode" }

// Detect returns true if the `claude` binary is in PATH.
func (a *Adapter) Detect() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// Install copies tokenmeter skill files into ~/.claude/skills/.
func (a *Adapter) Install(_ string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	return fs.WalkDir(skillsFS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := skillsFS.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(skillsDir, d.Name())
		return os.WriteFile(dest, data, 0o644)
	})
}

// Uninstall removes only the skill files we installed.
func (a *Adapter) Uninstall() error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".claude", "skills")
	for _, name := range skillNames {
		os.Remove(filepath.Join(skillsDir, name))
	}
	return nil
}

// Verify checks that ANTHROPIC_BASE_URL points to the proxy.
func (a *Adapter) Verify(proxyAddr string) error {
	url := os.Getenv("ANTHROPIC_BASE_URL")
	if !strings.Contains(url, proxyAddr) {
		return fmt.Errorf("ANTHROPIC_BASE_URL=%q does not contain %q — restart shell or run: source ~/.zshrc", url, proxyAddr)
	}
	return nil
}
