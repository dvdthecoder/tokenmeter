// Package vscode configures VS Code extensions (Cline) to route through tokenmeter.
package vscode

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

// Adapter configures VS Code (Cline extension).
type Adapter struct{}

func (a *Adapter) Name() string { return "vscode" }

// Detect returns true if `code` is in PATH or the VS Code user config dir exists.
func (a *Adapter) Detect() bool {
	if _, err := exec.LookPath("code"); err == nil {
		return true
	}
	_, err := os.Stat(userConfigDir())
	return err == nil
}

// Install patches VS Code settings.json with Cline's proxy endpoint.
func (a *Adapter) Install(proxyAddr string) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("vscode: create settings dir: %w", err)
	}

	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	baseURL := "http://" + proxyAddr + "/v1"
	settings["cline.apiProvider"] = "openai-compatible"
	settings["cline.openAiBaseUrl"] = baseURL
	// Route Copilot (and all HTTPS traffic) through tokenmeter's MITM proxy.
	settings["http.proxy"] = "http://" + proxyAddr
	settings["http.proxyStrictSSL"] = false

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("vscode: marshal settings: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Uninstall removes the Cline proxy settings.
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
	delete(settings, "cline.apiProvider")
	delete(settings, "cline.openAiBaseUrl")
	delete(settings, "http.proxy")
	delete(settings, "http.proxyStrictSSL")
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Verify checks the settings.json for the Cline proxy URL.
func (a *Adapter) Verify(proxyAddr string) error {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return fmt.Errorf("settings.json not found at %s", settingsPath())
	}
	if !strings.Contains(string(data), proxyAddr) {
		return fmt.Errorf("proxy address %q not found in %s", proxyAddr, settingsPath())
	}
	return nil
}

// userConfigDir returns the VS Code user config directory for the current OS.
func userConfigDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Code", "User")
		}
		return filepath.Join(home, "AppData", "Roaming", "Code", "User")
	default:
		return filepath.Join(home, ".config", "Code", "User")
	}
}

func settingsPath() string {
	return filepath.Join(userConfigDir(), "settings.json")
}
