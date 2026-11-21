package downloader

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsTransientFailure(t *testing.T) {
	transient := []string{
		"ERROR: unable to download webpage: <urlopen error [Errno -3] Temporary failure in name resolution>",
		"ERROR: HTTP Error 429: Too Many Requests",
		"ERROR: connection timed out",
		"ERROR: HTTP Error 503: Service Unavailable",
	}
	permanent := []string{
		"ERROR: unsupported URL",
		"ERROR: Video is private",
		"ERROR: requested format not available",
		"ERROR: Postprocessing: ffmpeg not found",
	}
	for _, msg := range transient {
		if !isTransientFailure(msg) {
			t.Errorf("isTransientFailure(%q) = false, want true", msg)
		}
	}
	for _, msg := range permanent {
		if isTransientFailure(msg) {
			t.Errorf("isTransientFailure(%q) = true, want false", msg)
		}
	}
}

func fakeRetryYTDLP(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func waitForTerminalStatus(t *testing.T, m *Manager, videoID string) Download {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	sawRetry := false
	for time.Now().Before(deadline) {
		for _, d := range m.List() {
			if d.VideoID != videoID {
				continue
			}
			if strings.Contains(d.Phase, "Retrying") {
				sawRetry = true
			}
			if d.Status == StatusDone || d.Status == StatusError || d.Status == StatusResumable {
				if !sawRetry && d.RetryAttempt > 0 {
					t.Error("download retried but no Retrying phase was observed")
				}
				return d
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("download did not reach a terminal status")
	return Download{}
}

func TestAutomaticRetrySucceedsAfterTransientFailures(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "attempts")
	final := filepath.Join(dir, "final.mp4")
	t.Setenv("YTUI_TEST_COUNTER", counter)
	t.Setenv("YTUI_TEST_FINAL", final)
	script := `#!/bin/sh
count=$(cat "$YTUI_TEST_COUNTER" 2>/dev/null || echo 0)
count=$((count + 1))
printf '%s' "$count" > "$YTUI_TEST_COUNTER"
if [ "$count" -lt 3 ]; then
  echo 'ERROR: unable to download webpage: Temporary failure in name resolution' >&2
  exit 1
fi
printf '[download] Destination: %s\n' "$YTUI_TEST_FINAL"
: > "$YTUI_TEST_FINAL"
`
	fakeRetryYTDLP(t, script)
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()
	opts := Options{RetryAttempts: 2, RetryBackoff: 200 * time.Millisecond}
	queued := m.Enqueue("retry-success", "Retry Test", "Channel", "https://example.invalid/v", opts)
	d := waitForTerminalStatus(t, m, queued.VideoID)
	if d.Status != StatusDone {
		t.Fatalf("status = %v, want done (error: %v)", d.Status, d.Err)
	}
	if d.RetryAttempt != 2 {
		t.Fatalf("RetryAttempt = %d, want 2", d.RetryAttempt)
	}
	data, readErr := os.ReadFile(counter)
	if readErr != nil || strings.TrimSpace(string(data)) != "3" {
		t.Fatalf("yt-dlp attempts = %q, want 3 (error: %v)", data, readErr)
	}
}

func TestPermanentFailureIsNotRetried(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "attempts")
	t.Setenv("YTUI_TEST_COUNTER", counter)
	script := `#!/bin/sh
count=$(cat "$YTUI_TEST_COUNTER" 2>/dev/null || echo 0)
count=$((count + 1))
printf '%s' "$count" > "$YTUI_TEST_COUNTER"
echo 'ERROR: unsupported URL' >&2
exit 1
`
	fakeRetryYTDLP(t, script)
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()
	opts := Options{RetryAttempts: 3, RetryBackoff: 10 * time.Millisecond}
	queued := m.Enqueue("permanent-failure", "Permanent", "Channel", "https://example.invalid/v", opts)
	d := waitForTerminalStatus(t, m, queued.VideoID)
	if d.Status != StatusError {
		t.Fatalf("status = %v, want error", d.Status)
	}
	if d.RetryAttempt != 0 {
		t.Fatalf("RetryAttempt = %d, want 0 for a permanent failure", d.RetryAttempt)
	}
	data, readErr := os.ReadFile(counter)
	if readErr != nil || strings.TrimSpace(string(data)) != "1" {
		t.Fatalf("yt-dlp attempts = %q, want 1 (error: %v)", data, readErr)
	}
}
