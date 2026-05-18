// Package opencode configures OpenCode to route through tokenmeter.
package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/backends"
)

func init() {
	backends.Register(&Adapter{})
}

// Adapter configures OpenCode.
type Adapter struct{}

func (a *Adapter) Name() string { return "opencode" }

// Detect returns true if `opencode` is in PATH or its config dir exists.
func (a *Adapter) Detect() bool {
	if _, err := exec.LookPath("opencode"); err == nil {
		return true
	}
	_, err := os.Stat(configDir())
	return err == nil
}

// Install patches ~/.config/opencode/config.json with the proxy baseURL.
// If the file does not exist yet, it is created with minimal config.
func (a *Adapter) Install(proxyAddr string) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("opencode: create config dir: %w", err)
	}

	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	baseURL := "http://" + proxyAddr
	setNestedKey(cfg, baseURL, "providers", "openai", "baseURL")
	setNestedKey(cfg, baseURL, "providers", "anthropic", "baseURL")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("opencode: marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Uninstall removes the proxy baseURL from the opencode config.
func (a *Adapter) Uninstall() error {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // nothing to do
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	deleteNestedKey(cfg, "providers", "openai", "baseURL")
	deleteNestedKey(cfg, "providers", "anthropic", "baseURL")
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Verify checks OPENAI_BASE_URL or the config file.
func (a *Adapter) Verify(proxyAddr string) error {
	url := os.Getenv("OPENAI_BASE_URL")
	if strings.Contains(url, proxyAddr) {
		return nil
	}
	// Fallback: check config file.
	data, err := os.ReadFile(configPath())
	if err == nil && strings.Contains(string(data), proxyAddr) {
		return nil
	}
	return fmt.Errorf("proxy not detected in OPENAI_BASE_URL or %s", configPath())
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// setNestedKey sets cfg[keys[0]][keys[1]]...[keys[n-1]] = value, creating
// intermediate maps as needed.
func setNestedKey(cfg map[string]any, value any, keys ...string) {
	for _, k := range keys[:len(keys)-1] {
		sub, ok := cfg[k].(map[string]any)
		if !ok {
			sub = map[string]any{}
			cfg[k] = sub
		}
		cfg = sub
	}
	cfg[keys[len(keys)-1]] = value
}

// deleteNestedKey removes cfg[keys[0]][keys[1]]...[keys[n-1]] if it exists.
func deleteNestedKey(cfg map[string]any, keys ...string) {
	for _, k := range keys[:len(keys)-1] {
		sub, ok := cfg[k].(map[string]any)
		if !ok {
			return
		}
		cfg = sub
	}
	delete(cfg, keys[len(keys)-1])
}
