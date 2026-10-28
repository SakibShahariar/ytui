//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package downloader

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func pauseProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil { return fmt.Errorf("download process is unavailable") }
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGSTOP); err != nil { return fmt.Errorf("pause download process group: %w", err) }
	return nil
}

func resumeProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil { return fmt.Errorf("download process is unavailable") }
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGCONT); err != nil { return fmt.Errorf("resume download process group: %w", err) }
	return nil
}

func killProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil { return nil }
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
