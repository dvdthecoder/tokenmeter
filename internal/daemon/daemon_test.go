package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// --- PID file tests ---

func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := os.Getenv("HOME")
	// Override HOME so DataDir/LogPath/PIDPath resolve into the temp dir.
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", orig) })
	return dir
}

func TestWriteReadPID(t *testing.T) {
	withTempDir(t)
	if err := WritePID(12345); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	data, err := os.ReadFile(PIDPath())
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	if string(data) != "12345" {
		t.Errorf("PID file content: got %q, want \"12345\"", data)
	}
}

func TestRemovePID(t *testing.T) {
	withTempDir(t)
	_ = WritePID(99)
	RemovePID()
	if _, err := os.Stat(PIDPath()); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

func TestReadPIDNoFile(t *testing.T) {
	withTempDir(t)
	pid, alive := ReadPID()
	if pid != 0 || alive {
		t.Errorf("expected (0, false), got (%d, %v)", pid, alive)
	}
}

func TestReadPIDCurrentProcess(t *testing.T) {
	withTempDir(t)
	myPID := os.Getpid()
	_ = WritePID(myPID)
	pid, alive := ReadPID()
	if pid != myPID || !alive {
		t.Errorf("expected (%d, true), got (%d, %v)", myPID, pid, alive)
	}
}

func TestReadPIDDeadProcess(t *testing.T) {
	withTempDir(t)
	// PID 999999999 is virtually guaranteed not to exist.
	_ = WritePID(999999999)
	_, alive := ReadPID()
	if alive {
		t.Error("expected dead process to be reported as not alive")
	}
}

// --- Shell patching tests ---

func TestPatchUnpatchShell(t *testing.T) {
	dir := withTempDir(t)
	// Force zsh so we get a deterministic RC path.
	origShell := os.Getenv("SHELL")
	os.Setenv("SHELL", "/bin/zsh")
	t.Cleanup(func() { os.Setenv("SHELL", origShell) })

	rc := filepath.Join(dir, ".zshrc")

	patched, err := PatchShell()
	if err != nil {
		t.Fatalf("PatchShell: %v", err)
	}
	if patched != rc {
		t.Errorf("expected RC path %s, got %s", rc, patched)
	}

	content, _ := os.ReadFile(rc)
	if !contains(string(content), "ANTHROPIC_BASE_URL") {
		t.Error("ANTHROPIC_BASE_URL not in RC file after patch")
	}
	if !contains(string(content), "OPENAI_BASE_URL") {
		t.Error("OPENAI_BASE_URL not in RC file after patch")
	}

	// Idempotent: patch again, no duplicates.
	_, _ = PatchShell()
	content2, _ := os.ReadFile(rc)
	if countOccurrences(string(content2), blockBegin) != 1 {
		t.Error("PatchShell is not idempotent — block appears more than once")
	}

	// Unpatch.
	_, err = UnpatchShell()
	if err != nil {
		t.Fatalf("UnpatchShell: %v", err)
	}
	content3, _ := os.ReadFile(rc)
	if contains(string(content3), "ANTHROPIC_BASE_URL") {
		t.Error("ANTHROPIC_BASE_URL still present after unpatch")
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	withTempDir(t)
	path, err := WriteDefaultConfig()
	if err != nil {
		t.Fatalf("WriteDefaultConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !contains(string(data), "127.0.0.1:4191") {
		t.Error("config missing proxy listen address")
	}
	// Second call must not overwrite (user edits preserved).
	_ = os.WriteFile(path, []byte("user content"), 0o600)
	_, _ = WriteDefaultConfig()
	data2, _ := os.ReadFile(path)
	if string(data2) != "user content" {
		t.Error("WriteDefaultConfig overwrote existing config")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
			i += len(sub) - 1
		}
	}
	return n
}
