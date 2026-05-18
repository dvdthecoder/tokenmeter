// Package daemon manages process lifecycle, PID files, and service installation.
package daemon

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the platform data directory for tokenmeter runtime files.
func DataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "tokenmeter")
	default: // linux, etc.
		return filepath.Join(home, ".local", "share", "tokenmeter")
	}
}

// ConfigDir returns the platform config directory.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tokenmeter")
}

// DefaultConfigPath is the config file written by install and read by the daemon.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// PIDPath is where the running daemon's PID is stored.
func PIDPath() string { return filepath.Join(DataDir(), "tokenmeter.pid") }

// LogPath is where the daemon's stdout/stderr are written.
func LogPath() string { return filepath.Join(DataDir(), "tokenmeter.log") }

// LaunchAgentPlist is the macOS launchd plist path (user agent).
func LaunchAgentPlist() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.tokenmeter.plist")
}

// SystemdUserUnit is the Linux systemd user unit path.
func SystemdUserUnit() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "tokenmeter.service")
}

func ensureDataDir() error {
	return os.MkdirAll(DataDir(), 0o700)
}
