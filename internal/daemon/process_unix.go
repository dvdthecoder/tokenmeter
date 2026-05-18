//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Start launches tokenmeter start as a detached background process, writes the
// PID file, and returns the new PID. Logs go to LogPath().
func Start(binary, configPath string) (int, error) {
	if err := AssertNotRunning(); err != nil {
		return 0, err
	}
	if err := ensureDataDir(); err != nil {
		return 0, err
	}

	logFile, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()

	args := []string{"start"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	// Setsid creates a new session so the child survives the parent's exit.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start process: %w", err)
	}
	pid := cmd.Process.Pid
	// Release ownership — child runs independently.
	_ = cmd.Process.Release()

	if err := WritePID(pid); err != nil {
		return pid, err
	}
	return pid, nil
}

// Stop sends SIGTERM to the running daemon and waits up to 10 s for it to exit.
func Stop() error {
	pid, alive := ReadPID()
	if !alive {
		RemovePID()
		return fmt.Errorf("tokenmeter is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	// Poll for exit (max 10 s).
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			// Process is gone.
			RemovePID()
			return nil
		}
	}

	// Force-kill if it didn't respond to SIGTERM.
	_ = proc.Kill()
	RemovePID()
	return fmt.Errorf("daemon did not exit cleanly within 10s — force-killed")
}
