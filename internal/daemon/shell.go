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
// It starts the daemon in the background if it is not already running, then
// sets the proxy env vars. The pgrep check avoids spawning duplicate daemons;
// the nohup ensures the daemon outlives the shell that starts it.
const envBlock = `# tokenmeter — begin
pgrep -qf "tokenmeter start" 2>/dev/null || nohup tokenmeter start >/dev/null 2>&1 &
export ANTHROPIC_BASE_URL=http://127.0.0.1:4191
export OPENAI_BASE_URL=http://127.0.0.1:4191
# tokenmeter — end`

// fishBlock is the fish-shell equivalent.
const fishBlock = `# tokenmeter — begin
pgrep -qf "tokenmeter start" >/dev/null 2>&1; or nohup tokenmeter start >/dev/null 2>&1 &
set -gx ANTHROPIC_BASE_URL http://127.0.0.1:4191
set -gx OPENAI_BASE_URL http://127.0.0.1:4191
# tokenmeter — end`

// PatchShell writes the tokenmeter env-var block to the user's shell RC file.
// Idempotent: if the current block is already present verbatim, it does
// nothing. If an older block is present (e.g. from a version before an
// envBlock change), it replaces it so upgrades actually take effect.
// Returns the path of the file that was (or would be) patched.
func PatchShell() (string, error) {
	rc, block, err := shellRCFile()
	if err != nil {
		return "", err
	}

	existing, _ := os.ReadFile(rc)
	if strings.Contains(string(existing), block) {
		return rc, nil // already patched with the current block
	}

	base := strings.TrimRight(stripBlock(string(existing)), "\n")
	updated := block + "\n"
	if base != "" {
		updated = base + "\n\n" + updated
	}

	if err := os.WriteFile(rc, []byte(updated), 0o644); err != nil {
		return rc, fmt.Errorf("patch shell %s: %w", rc, err)
	}
	return rc, nil
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

	result := strings.TrimRight(stripBlock(string(content)), "\n") + "\n"
	return rc, os.WriteFile(rc, []byte(result), 0o644)
}

// stripBlock removes any tokenmeter env-var block (old or new) from content,
// identified by the blockBegin/blockEnd marker comments.
func stripBlock(content string) string {
	lines := strings.Split(content, "\n")
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
	return strings.Join(keep, "\n")
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
