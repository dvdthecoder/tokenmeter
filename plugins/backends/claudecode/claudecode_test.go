package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCopiesSkillFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &Adapter{}
	if err := a.Install("127.0.0.1:4191"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, name := range skillNames {
		path := filepath.Join(home, ".claude", "skills", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("skill file missing: %s", name)
		}
	}
}

func TestUninstallRemovesSkillFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &Adapter{}
	_ = a.Install("127.0.0.1:4191")
	_ = a.Uninstall()

	for _, name := range skillNames {
		path := filepath.Join(home, ".claude", "skills", name)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("skill file still present after uninstall: %s", name)
		}
	}
}

func TestVerifyOK(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:4191")
	a := &Adapter{}
	if err := a.Verify("127.0.0.1:4191"); err != nil {
		t.Errorf("expected OK, got: %v", err)
	}
}

func TestVerifyFail(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	a := &Adapter{}
	if err := a.Verify("127.0.0.1:4191"); err == nil {
		t.Error("expected failure when ANTHROPIC_BASE_URL does not contain proxy addr")
	}
}
