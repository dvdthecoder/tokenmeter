package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes pid to the PID file, creating DataDir if needed.
func WritePID(pid int) error {
	if err := ensureDataDir(); err != nil {
		return err
	}
	return os.WriteFile(PIDPath(), []byte(strconv.Itoa(pid)), 0o600)
}

// ReadPID returns the PID in the PID file and whether that process is alive.
func ReadPID() (pid int, alive bool) {
	data, err := os.ReadFile(PIDPath())
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}
	// Signal 0 checks process existence without delivering a signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}
	return pid, true
}

// RemovePID deletes the PID file. Ignores missing-file errors.
func RemovePID() {
	os.Remove(PIDPath())
}

// AssertNotRunning returns an error if tokenmeter is already running.
func AssertNotRunning() error {
	if pid, alive := ReadPID(); alive {
		return fmt.Errorf("tokenmeter is already running (pid %d)", pid)
	}
	return nil
}
