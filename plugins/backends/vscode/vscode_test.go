package vscode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}
	return home
}

func TestInstallPatchesSettings(t *testing.T) {
	home := tempHome(t)
	_ = home

	a := &Adapter{}
	if err := a.Install("127.0.0.1:4191"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(settingsPath())
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if settings["cline.apiProvider"] != "openai-compatible" {
		t.Errorf("apiProvider not set: %v", settings["cline.apiProvider"])
	}
	want := "http://127.0.0.1:4191/v1"
	if settings["cline.openAiBaseUrl"] != want {
		t.Errorf("openAiBaseUrl: got %v, want %s", settings["cline.openAiBaseUrl"], want)
	}
}

func TestInstallMergesExistingSettings(t *testing.T) {
	tempHome(t)
	path := settingsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	existing := `{"editor.fontSize":14,"cline.apiKey":"sk-test"}`
	_ = os.WriteFile(path, []byte(existing), 0o600)

	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")

	data, _ := os.ReadFile(path)
	var settings map[string]any
	_ = json.Unmarshal(data, &settings)

	if settings["editor.fontSize"] != float64(14) {
		t.Error("existing key 'editor.fontSize' was overwritten")
	}
	if settings["cline.apiKey"] != "sk-test" {
		t.Error("existing cline.apiKey was overwritten")
	}
}

func TestUninstallRemovesClineSettings(t *testing.T) {
	tempHome(t)

	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")
	_ = a.Uninstall()

	data, _ := os.ReadFile(settingsPath())
	var settings map[string]any
	_ = json.Unmarshal(data, &settings)

	if _, ok := settings["cline.openAiBaseUrl"]; ok {
		t.Error("cline.openAiBaseUrl still present after uninstall")
	}
}

func TestVerifyOK(t *testing.T) {
	tempHome(t)
	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")
	if err := a.Verify("127.0.0.1:4191"); err != nil {
		t.Errorf("expected OK: %v", err)
	}
}
