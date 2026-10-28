package notifier

import (
	"os/exec"
	"strings"
)

// Send issues a best-effort Linux desktop notification through notify-send.
// Missing notification services never affect the operation that triggered it.
func Send(title, body string) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		title = "ytui"
	}
	cmd := exec.Command("notify-send", "--app-name=ytui", title, body)
	if err := cmd.Start(); err == nil {
		_ = cmd.Process.Release()
	}
}
