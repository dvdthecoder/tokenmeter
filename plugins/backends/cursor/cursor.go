// Package cursor configures Cursor to route through tokenmeter.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/backends"
)

func init() {
	backends.Register(&Adapter{})
}

// Adapter configures Cursor.
type Adapter struct{}

func (a *Adapter) Name() string { return "cursor" }

// Detect returns true if the `cursor` binary is in PATH or the Cursor user
// config directory exists.
func (a *Adapter) Detect() bool {
	if _, err := exec.LookPath("cursor"); err == nil {
		return true
	}
	_, err := os.Stat(userConfigDir())
	return err == nil
}

// Install patches Cursor's settings.json so that HTTPS CONNECT traffic
// (e.g. GitHub Copilot) routes through tokenmeter's MITM proxy.
// ANTHROPIC_BASE_URL / OPENAI_BASE_URL are set globally by PatchShell and
// cover Cursor's built-in AI and extension API calls automatically.
func (a *Adapter) Install(proxyAddr string) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cursor: create settings dir: %w", err)
	}

	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	settings["http.proxy"] = "http://" + proxyAddr
	settings["http.proxyStrictSSL"] = false

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cursor: marshal settings: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Uninstall removes the proxy settings from Cursor's settings.json.
func (a *Adapter) Uninstall() error {
	path := settingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	settings := map[string]any{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}
	delete(settings, "http.proxy")
	delete(settings, "http.proxyStrictSSL")
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Verify checks that the proxy address is present in Cursor's settings.json.
func (a *Adapter) Verify(proxyAddr string) error {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return fmt.Errorf("cursor settings.json not found at %s", settingsPath())
	}
	if !strings.Contains(string(data), proxyAddr) {
		return fmt.Errorf("proxy address %q not found in %s", proxyAddr, settingsPath())
	}
	return nil
}

func userConfigDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Cursor", "User")
		}
		return filepath.Join(home, "AppData", "Roaming", "Cursor", "User")
	default:
		return filepath.Join(home, ".config", "Cursor", "User")
	}
}

func settingsPath() string {
	return filepath.Join(userConfigDir(), "settings.json")
}
