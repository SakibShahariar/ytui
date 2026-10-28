package downloader

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildArgs(t *testing.T) {
	destDir := filepath.Join("tmp", "downloads")
	outputTemplate := filepath.Join(destDir, "%(title).100s [%(id)s].%(ext)s")
	videoURL := "https://example.invalid/video"

	tests := []struct {
		name string
		download Download
		want []string
	}{
		{
			name: "defaults to best video and audio",
			download: Download{URL: videoURL},
			want: []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate, "-f", "bestvideo+bestaudio/best", videoURL},
		},
		{
			name: "video options",
			download: Download{URL: videoURL, Opts: Options{
				Format: "bestvideo[height<=1080]+bestaudio/best[height<=1080]",
				WriteSubs: true, EmbedThumbnail: true, CookiesFromBrowser: "firefox",
			}},
			want: []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate,
				"-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]",
				"--write-subs", "--sub-langs", "en.*,en", "--embed-subs", "--embed-thumbnail",
				"--cookies-from-browser", "firefox", videoURL},
		},
		{
			name: "audio extraction ignores video format",
			download: Download{URL: videoURL, Opts: Options{
				Format: "should-not-be-used", ExtractAudio: true, AudioFormat: "opus",
			}},
			want: []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate, "-x", "--audio-format", "opus", videoURL},
		},
		{
			name: "audio extraction leaves format to yt-dlp when unspecified",
			download: Download{URL: videoURL, Opts: Options{ExtractAudio: true}},
			want: []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate, "-x", videoURL},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildArgs(destDir, &tt.download); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

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

func TestRunIncludesStderrInFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}

	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	script := "#!/bin/sh\nprintf '%s\\n' 'fake diagnostic from stderr' >&2\nexit 1\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &Download{
		VideoID: "stderr-test",
		URL:     "https://example.invalid/video",
		Status:  StatusQueued,
	}
	m := &Manager{
		byID:    map[string]*Download{"stderr-test": d},
		order:   []string{"stderr-test"},
		destDir: t.TempDir(),
	}

	m.run(d)
	if d.Status != StatusError {
		t.Fatalf("status after failing yt-dlp = %v, want error", d.Status)
	}
	if d.Err == nil || !strings.Contains(d.Err.Error(), "fake diagnostic from stderr") {
		t.Fatalf("error = %v, want stderr diagnostic", d.Err)
	}
}

func TestRunDrainsOversizedOutputLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}

	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	script := "#!/bin/sh\nprintf '%1100000s\\n' x\nprintf '%s\\n' 'diagnostic after oversized line' >&2\nexit 1\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &Download{VideoID: "oversized-test", URL: "https://example.invalid/video", Status: StatusQueued}
	m := &Manager{byID: map[string]*Download{"oversized-test": d}, order: []string{"oversized-test"}, destDir: t.TempDir()}
	done := make(chan struct{})
	go func() {
		m.run(d)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		m.Cancel(d.VideoID)
		t.Fatal("download worker hung while draining oversized yt-dlp output")
	}
	if d.Status != StatusError {
		t.Fatalf("status after failing yt-dlp = %v, want error", d.Status)
	}
	if d.Err == nil || !strings.Contains(d.Err.Error(), "diagnostic after oversized line") {
		t.Fatalf("error = %v, want diagnostic after oversized line", d.Err)
	}
}
