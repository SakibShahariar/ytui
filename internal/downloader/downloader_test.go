package downloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProgressRegex(t *testing.T) {
	line := "[download]  42.3% of  123.45MiB at  5.67MiB/s ETA 00:12"
	match := progressRe.FindStringSubmatch(line)
	if len(match) != 4 {
		t.Fatalf("progress regex did not match sample: %q", line)
	}
	if match[1] != "42.3" || match[2] != "5.67MiB/s" || match[3] != "00:12" {
		t.Errorf("unexpected progress captures: %#v", match[1:])
	}
}

func TestDestinationRegex(t *testing.T) {
	line := "[download] Destination: /tmp/video title.webm"
	match := destRe.FindStringSubmatch(line)
	if len(match) != 2 {
		t.Fatalf("destination regex did not match sample: %q", line)
	}
	if match[1] != "/tmp/video title.webm" {
		t.Errorf("destination = %q, want full path", match[1])
	}
}

func TestRunDoesNotRestartCanceledDownload(t *testing.T) {
	d := &Download{VideoID: "video", Status: StatusCanceled}
	m := &Manager{
		byID:    map[string]*Download{"video": d},
		order:   []string{"video"},
		destDir: t.TempDir(),
	}

	m.run(d)
	if d.Status != StatusCanceled {
		t.Errorf("status after run = %v, want canceled", d.Status)
	}
	if d.cmd != nil {
		t.Error("canceled download unexpectedly created a process")
	}
}


func TestShutdownPreventsNewDownloads(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Shutdown()
	d := m.Enqueue("video", "title", "channel", "https://example.com/video", Options{})
	if d.Status != StatusCanceled {
		t.Fatalf("Enqueue() after Shutdown status = %v, want canceled", d.Status)
	}
	if m.Active() {
		t.Fatal("manager reports active downloads after Shutdown")
	}
}


func TestCleanupStalePartialsOnlyRemovesOldArtifacts(t *testing.T) {
	dir := t.TempDir()
	oldPart := filepath.Join(dir, "video [abc].mp4.part")
	oldYtdl := filepath.Join(dir, "video [def].ytdl")
	freshPart := filepath.Join(dir, "fresh [ghi].mp4.part")
	ordinary := filepath.Join(dir, "completed.mp4")
	for _, path := range []string{oldPart, oldYtdl, freshPart, ordinary} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	for _, path := range []string{oldPart, oldYtdl} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := CleanupStalePartials(dir, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupStalePartials() error = %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	for _, path := range []string{oldPart, oldYtdl} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale artifact %q remains, stat err = %v", path, err)
		}
	}
	for _, path := range []string{freshPart, ordinary} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("non-stale or completed file %q was removed: %v", path, err)
		}
	}
}

func TestHasPartialForVideo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "title [video123].webm.part")
	if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !hasPartialForVideo(dir, "video123") {
		t.Fatal("hasPartialForVideo() = false for matching partial file")
	}
	if hasPartialForVideo(dir, "other") {
		t.Fatal("hasPartialForVideo() = true for unrelated video")
	}
}

func TestCancelQueuedDownloadMarksItCanceled(t *testing.T) {
	d := &Download{VideoID: "video", Status: StatusQueued}
	m := &Manager{
		byID:    map[string]*Download{"video": d},
		order:   []string{"video"},
		destDir: t.TempDir(),
	}

	m.Cancel("video")
	if d.Status != StatusCanceled {
		t.Fatalf("status after Cancel() = %v, want canceled", d.Status)
	}
	if d.cmd != nil {
		t.Fatal("Cancel() unexpectedly created a process")
	}

	// A worker that has not started yet must observe cancellation and exit
	// without launching yt-dlp.
	m.run(d)
	if d.Status != StatusCanceled {
		t.Errorf("status after run() = %v, want canceled", d.Status)
	}
	if d.cmd != nil {
		t.Error("canceled queued download unexpectedly created a process")
	}
}
