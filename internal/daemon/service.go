package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallService installs tokenmeter as a system service (launchd on macOS,
// systemd user unit on Linux) and starts it.
func InstallService(binary, configPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(binary, configPath)
	case "linux":
		return installSystemd(binary, configPath)
	default:
		return fmt.Errorf("system service not supported on %s — run 'tokenmeter daemon' manually", runtime.GOOS)
	}
}

// UninstallService stops and removes the system service.
func UninstallService() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		return fmt.Errorf("system service not supported on %s", runtime.GOOS)
	}
}

// --- macOS launchd ---

func launchdPlist(binary, configPath, logPath string) string {
	var configArgs string
	if configPath != "" {
		configArgs = fmt.Sprintf("\n        <string>--config</string>\n        <string>%s</string>", configPath)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tokenmeter</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, binary, configArgs, logPath, logPath)
}

func installLaunchd(binary, configPath string) error {
	plistPath := LaunchAgentPlist()
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	if err := ensureDataDir(); err != nil {
		return err
	}

	content := launchdPlist(binary, configPath, LogPath())
	if err := os.WriteFile(plistPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// Unload first in case it was already loaded (idempotent re-install).
	_ = runCmd("launchctl", "unload", plistPath)
	if err := runCmd("launchctl", "load", "-w", plistPath); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

func uninstallLaunchd() error {
	plistPath := LaunchAgentPlist()
	_ = runCmd("launchctl", "unload", "-w", plistPath)
	return os.Remove(plistPath)
}

// --- Linux systemd user unit ---

func systemdUnit(binary, configPath string) string {
	execStart := binary + " start"
	if configPath != "" {
		execStart += " --config " + configPath
	}
	return fmt.Sprintf(`[Unit]
Description=tokenmeter — LLM API token usage meter
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, execStart, LogPath(), LogPath())
}

func installSystemd(binary, configPath string) error {
	unitPath := SystemdUserUnit()
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	if err := ensureDataDir(); err != nil {
		return err
	}

	content := systemdUnit(binary, configPath)
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}

	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	if err := runCmd("systemctl", "--user", "enable", "--now", "tokenmeter"); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}
	return nil
}

func uninstallSystemd() error {
	_ = runCmd("systemctl", "--user", "disable", "--now", "tokenmeter")
	return os.Remove(SystemdUserUnit())
}

// runCmd executes a system command, returning an error that includes stderr.
func runCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
