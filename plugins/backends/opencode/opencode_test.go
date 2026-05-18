package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCreatesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &Adapter{}
	if err := a.Install("127.0.0.1:4191"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "config.json"))
	if err != nil {
		t.Fatalf("config not created: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	providers, _ := cfg["providers"].(map[string]any)
	openai, _ := providers["openai"].(map[string]any)
	if openai["baseURL"] != "http://127.0.0.1:4191" {
		t.Errorf("unexpected baseURL: %v", openai["baseURL"])
	}
}

func TestInstallMergesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Pre-existing config with user data.
	cfgDir := filepath.Join(home, ".config", "opencode")
	_ = os.MkdirAll(cfgDir, 0o755)
	existing := `{"theme":"dark","providers":{"openai":{"apiKey":"sk-test"}}}`
	_ = os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(existing), 0o600)

	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")

	data, _ := os.ReadFile(filepath.Join(cfgDir, "config.json"))
	var cfg map[string]any
	_ = json.Unmarshal(data, &cfg)

	if cfg["theme"] != "dark" {
		t.Error("existing key 'theme' was overwritten")
	}
	providers, _ := cfg["providers"].(map[string]any)
	openai, _ := providers["openai"].(map[string]any)
	if openai["apiKey"] != "sk-test" {
		t.Error("existing apiKey was overwritten")
	}
	if openai["baseURL"] != "http://127.0.0.1:4191" {
		t.Errorf("baseURL not set: %v", openai["baseURL"])
	}
}

func TestUninstallRemovesBaseURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")
	_ = a.Uninstall()

	data, _ := os.ReadFile(filepath.Join(home, ".config", "opencode", "config.json"))
	var cfg map[string]any
	_ = json.Unmarshal(data, &cfg)
	providers, _ := cfg["providers"].(map[string]any)
	openai, _ := providers["openai"].(map[string]any)
	if _, ok := openai["baseURL"]; ok {
		t.Error("baseURL still present after uninstall")
	}
}
