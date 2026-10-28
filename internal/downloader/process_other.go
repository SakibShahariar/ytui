//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package downloader

import (
	"fmt"
	"os/exec"
)

func configureProcessGroup(cmd *exec.Cmd) {}

func pauseProcess(cmd *exec.Cmd) error {
	return fmt.Errorf("pausing active downloads is not supported on this platform")
}

func resumeProcess(cmd *exec.Cmd) error {
	return fmt.Errorf("resuming active downloads is not supported on this platform")
}
