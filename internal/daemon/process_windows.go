//go:build windows

package daemon

import "fmt"

func Start(binary, configPath string) (int, error) {
	return 0, fmt.Errorf("background daemon not yet supported on Windows — run 'tokenmeter start' in a terminal or use Task Scheduler")
}

func Stop() error {
	return fmt.Errorf("stop not yet supported on Windows — terminate the process manually")
}
