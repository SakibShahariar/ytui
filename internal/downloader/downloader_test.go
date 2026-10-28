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
				WriteSubs: true, SubtitleLang: "fr", EmbedThumbnail: true, CookiesFromBrowser: "firefox",
			}},
			want: []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate,
				"-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]",
				"--write-subs", "--write-auto-subs", "--sub-langs", "fr", "--embed-thumbnail",
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

func TestCancelActiveDownloadStopsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}

	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	if err := os.WriteFile(fakeYTDLP, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &Download{VideoID: "active-cancel-test", URL: "https://example.invalid/video", Status: StatusQueued}
	m := &Manager{byID: map[string]*Download{d.VideoID: d}, order: []string{d.VideoID}, destDir: t.TempDir()}
	done := make(chan struct{})
	go func() {
		m.run(d)
		close(done)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		started := d.cmd != nil && d.cmd.Process != nil
		m.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			m.Cancel(d.VideoID)
			t.Fatal("fake yt-dlp process did not start")
		}
		select {
		case <-done:
			t.Fatal("download worker exited before the fake process could be canceled")
		case <-time.After(10 * time.Millisecond):
		}
	}

	m.Cancel(d.VideoID)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("download worker did not stop after cancellation")
	}
	if d.Status != StatusCanceled {
		t.Fatalf("status after cancellation = %v, want canceled", d.Status)
	}
}


func TestMoveQueuedChangesPendingAndVisibleOrder(t *testing.T) {
	first := &Download{VideoID: "first", Status: StatusQueued}
	second := &Download{VideoID: "second", Status: StatusQueued}
	third := &Download{VideoID: "third", Status: StatusQueued}
	m := &Manager{
		byID: map[string]*Download{"first": first, "second": second, "third": third},
		order: []string{"first", "second", "third"},
		pending: []string{"first", "second", "third"},
		wake: make(chan struct{}, 1),
	}
	if !m.MoveQueued("third", -1) { t.Fatal("MoveQueued(third, -1) = false, want true") }
	if got, want := strings.Join(m.pending, ","), "first,third,second"; got != want { t.Fatalf("pending = %q, want %q", got, want) }
	if got, want := strings.Join(m.ListIDsForTest(), ","), "first,third,second"; got != want { t.Fatalf("visible order = %q, want %q", got, want) }
	if m.MoveQueued("first", -1) { t.Fatal("moving first item up unexpectedly succeeded") }
	if m.MoveQueued("second", 0) { t.Fatal("invalid move delta unexpectedly succeeded") }
}

func (m *Manager) ListIDsForTest() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.order...)
}

func TestPauseResumeQueuedDownload(t *testing.T) {
	d := &Download{VideoID: "video", Status: StatusQueued}
	m := &Manager{
		byID: map[string]*Download{"video": d},
		order: []string{"video"},
		pending: []string{"video"},
		wake: make(chan struct{}, 1),
	}
	if err := m.Pause("video"); err != nil { t.Fatalf("Pause() error = %v", err) }
	if d.Status != StatusPaused { t.Fatalf("status after Pause() = %v, want paused", d.Status) }
	if len(m.pending) != 0 { t.Fatalf("pending after Pause() = %v, want empty", m.pending) }
	if err := m.Resume("video"); err != nil { t.Fatalf("Resume() error = %v", err) }
	if d.Status != StatusQueued { t.Fatalf("status after Resume() = %v, want queued", d.Status) }
	if len(m.pending) != 1 || m.pending[0] != "video" { t.Fatalf("pending after Resume() = %v, want [video]", m.pending) }
	select { case <-m.wake: default: t.Fatal("Resume() did not wake scheduler") }
}

func TestPauseRejectsActiveDownload(t *testing.T) {
	d := &Download{VideoID: "active", Status: StatusDownloading}
	m := &Manager{byID: map[string]*Download{"active": d}, wake: make(chan struct{}, 1)}
	if err := m.Pause("active"); err == nil { t.Fatal("Pause() unexpectedly accepted an active download") }
	if d.Status != StatusDownloading { t.Fatalf("status changed to %v after rejected Pause()", d.Status) }
}


func TestPersistedQueueRestoresOrderAndPausedState(t *testing.T) {
	queueFile := filepath.Join(t.TempDir(), ".ytui-queue.json")
	queued := &Download{VideoID: "queued", Title: "Queued", URL: "https://example.invalid/queued", Status: StatusQueued, Opts: Options{QualityLabel: "1080p"}}
	paused := &Download{VideoID: "paused", Title: "Paused", URL: "https://example.invalid/paused", Status: StatusPaused}
	m := &Manager{
		byID: map[string]*Download{"queued": queued, "paused": paused},
		order: []string{"queued", "paused"},
		pending: []string{"queued"},
		queueFile: queueFile,
	}
	m.mu.Lock()
	err := m.persistQueueLocked()
	m.mu.Unlock()
	if err != nil { t.Fatalf("persistQueueLocked() error = %v", err) }
	info, err := os.Stat(queueFile)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0o600 { t.Fatalf("queue file mode = %04o, want 0600", info.Mode().Perm()) }

	restored := &Manager{byID: make(map[string]*Download), queueFile: queueFile}
	if err := restored.loadQueue(); err != nil { t.Fatalf("loadQueue() error = %v", err) }
	if got, want := strings.Join(restored.order, ","), "queued,paused"; got != want { t.Fatalf("restored order = %q, want %q", got, want) }
	if len(restored.pending) != 1 || restored.pending[0] != "queued" { t.Fatalf("restored pending = %v, want [queued]", restored.pending) }
	if got := restored.byID["paused"].Status; got != StatusPaused { t.Fatalf("restored paused status = %v, want paused", got) }
	if got := restored.byID["queued"].Opts.QualityLabel; got != "1080p" { t.Fatalf("restored quality = %q, want 1080p", got) }
}


func TestPauseResumeActiveDownload(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("process-group pause is not supported on Windows") }
	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	if err := os.WriteFile(fakeYTDLP, []byte("#!/bin/sh\nwhile :; do sleep 1; done\n"), 0o700); err != nil { t.Fatal(err) }
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := &Download{VideoID:"active-pause-test", URL:"https://example.invalid/video", Status:StatusQueued}
	m := &Manager{byID:map[string]*Download{d.VideoID:d}, order:[]string{d.VideoID}, destDir:t.TempDir()}
	done := make(chan struct{})
	go func(){ m.run(d); close(done) }()
	deadline := time.Now().Add(3*time.Second)
	for {
		m.mu.Lock()
		started := d.cmd != nil && d.cmd.Process != nil
		m.mu.Unlock()
		if started { break }
		if time.Now().After(deadline) { t.Fatal("fake yt-dlp process did not start") }
		select { case <-done: t.Fatal("download exited before pause test"); case <-time.After(10*time.Millisecond): }
	}
	if err := m.Pause(d.VideoID); err != nil { m.Cancel(d.VideoID); t.Fatalf("Pause() error = %v", err) }
	m.mu.Lock()
	paused := d.Status == StatusPaused
	m.mu.Unlock()
	if !paused { m.Cancel(d.VideoID); t.Fatal("active download status did not become paused") }
	if err := m.Resume(d.VideoID); err != nil { m.Cancel(d.VideoID); t.Fatalf("Resume() error = %v", err) }
	m.mu.Lock()
	resumed := d.Status == StatusDownloading
	m.mu.Unlock()
	if !resumed { m.Cancel(d.VideoID); t.Fatal("active download status did not return to downloading") }
	m.Cancel(d.VideoID)
	select { case <-done: case <-time.After(3*time.Second): t.Fatal("download did not exit after cancellation") }
}


func TestBuildArgsUsesEnglishSubtitleFallback(t *testing.T) {
	args := buildArgs(t.TempDir(), &Download{URL:"https://example.invalid/video", Opts:Options{WriteSubs:true}})
	want := []string{"--write-subs", "--write-auto-subs", "--sub-langs", "en.*,en"}
	for i := 0; i < len(want); i++ {
		found := false
		for j := 0; j+len(want) <= len(args); j++ {
			match := true
			for k := range want { if args[j+k] != want[k] { match = false; break } }
			if match { found = true; break }
		}
		if found { return }
		t.Fatalf("buildArgs() = %#v, want subtitle flags %#v", args, want)
	}
}
