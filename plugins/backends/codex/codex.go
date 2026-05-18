// Package codex configures the Codex CLI to route through tokenmeter.
package codex

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/backends"
)

func init() {
	backends.Register(&Adapter{})
}

// Adapter configures Codex CLI.
type Adapter struct{}

func (a *Adapter) Name() string { return "codex" }

// Detect returns true if the `codex` binary is in PATH.
func (a *Adapter) Detect() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

// Install is a no-op: Codex reads OPENAI_BASE_URL which daemon.PatchShell already sets.
func (a *Adapter) Install(_ string) error { return nil }

// Uninstall is a no-op: env var cleanup is handled by daemon.UnpatchShell.
func (a *Adapter) Uninstall() error { return nil }

// Verify checks that OPENAI_BASE_URL points to the proxy.
func (a *Adapter) Verify(proxyAddr string) error {
	url := os.Getenv("OPENAI_BASE_URL")
	if !strings.Contains(url, proxyAddr) {
		return fmt.Errorf("OPENAI_BASE_URL=%q does not contain %q — restart shell or run: source ~/.zshrc", url, proxyAddr)
	}
	return nil
}
