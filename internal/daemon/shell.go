package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	blockBegin = "# tokenmeter — begin"
	blockEnd   = "# tokenmeter — end"
)

// envBlock is injected into the shell RC file by PatchShell.
// Both vars point to the loopback proxy so every tool is intercepted.
const envBlock = `# tokenmeter — begin
export ANTHROPIC_BASE_URL=http://127.0.0.1:4191
export OPENAI_BASE_URL=http://127.0.0.1:4191
# tokenmeter — end`

// fishBlock is the fish-shell equivalent.
const fishBlock = `# tokenmeter — begin
set -gx ANTHROPIC_BASE_URL http://127.0.0.1:4191
set -gx OPENAI_BASE_URL http://127.0.0.1:4191
# tokenmeter — end`

// PatchShell appends the tokenmeter env-var block to the user's shell RC file.
// Idempotent: if the block is already present, it does nothing.
// Returns the path of the file that was (or would be) patched.
func PatchShell() (string, error) {
	rc, block, err := shellRCFile()
	if err != nil {
		return "", err
	}

	existing, _ := os.ReadFile(rc)
	if strings.Contains(string(existing), blockBegin) {
		return rc, nil // already patched
	}

	f, err := os.OpenFile(rc, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return rc, fmt.Errorf("patch shell %s: %w", rc, err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n%s\n", block)
	return rc, err
}

// UnpatchShell removes the tokenmeter env-var block from the shell RC file.
func UnpatchShell() (string, error) {
	rc, _, err := shellRCFile()
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(rc)
	if err != nil {
		if os.IsNotExist(err) {
			return rc, nil
		}
		return rc, err
	}

	lines := strings.Split(string(content), "\n")
	var keep []string
	inBlock := false
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			inBlock = true
		case blockEnd:
			inBlock = false
		default:
			if !inBlock {
				keep = append(keep, line)
			}
		}
	}

	// Trim trailing blank lines added by PatchShell.
	result := strings.TrimRight(strings.Join(keep, "\n"), "\n") + "\n"
	return rc, os.WriteFile(rc, []byte(result), 0o644)
}

// shellRCFile returns the RC file path and the appropriate env block for the
// user's current shell ($SHELL env var). Falls back to ~/.profile.
func shellRCFile() (path, block string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc"), envBlock, nil
	case "bash":
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), envBlock, nil
		}
		return filepath.Join(home, ".bashrc"), envBlock, nil
	case "fish":
		dir := filepath.Join(home, ".config", "fish", "conf.d")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", "", err
		}
		return filepath.Join(dir, "tokenmeter.fish"), fishBlock, nil
	default:
		return filepath.Join(home, ".profile"), envBlock, nil
	}
}
