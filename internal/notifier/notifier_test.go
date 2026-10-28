package notifier

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSendMissingNotifySendDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	Send("Download complete", "Example")
}

func TestSendUsesNotifySendWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "called")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"" + marker + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "notify-send"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	Send("Download complete", "Example")
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("notify-send stub was not invoked")
}
