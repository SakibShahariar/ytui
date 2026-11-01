package downloader

import (
	"encoding/json"
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

func TestParseProgressLineSupportsOptionalFieldsAndFragments(t *testing.T) {
	tests := []struct {
		name string
		line string
		wantProgress float64
		wantSpeed string
		wantETA string
		wantOK bool
	}{
		{name: "standard", line: "[download]  42.3% of 123MiB at 5.67MiB/s ETA 00:12", wantProgress: 42.3, wantSpeed: "5.67MiB/s", wantETA: "00:12", wantOK: true},
		{name: "missing speed and eta", line: "[download]  42.3% of 123MiB", wantProgress: 42.3, wantOK: true},
		{name: "speed without eta", line: "[download]  42.3% of 123MiB at 5.67MiB/s", wantProgress: 42.3, wantSpeed: "5.67MiB/s", wantOK: true},
		{name: "eta without speed", line: "[download]  42.3% of 123MiB ETA 00:12", wantProgress: 42.3, wantETA: "00:12", wantOK: true},
		{name: "fragment download", line: "[download]  12.5% of ~8.00MiB at 1.20MiB/s ETA 00:06 (frag 2/16)", wantProgress: 12.5, wantSpeed: "1.20MiB/s", wantETA: "00:06", wantOK: true},
		{name: "completed", line: "[download] 100% of 123MiB", wantProgress: 100, wantOK: true},
		{name: "not a progress line", line: "[Merger] Merging formats into output.mp4", wantOK: false},
		{name: "malformed percentage", line: "[download] --% of 123MiB", wantOK: false},
		{name: "out of range", line: "[download] 120% of 123MiB", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress, speed, eta, ok := parseProgressLine(tt.line)
			if ok != tt.wantOK { t.Fatalf("parseProgressLine() ok = %v, want %v", ok, tt.wantOK) }
			if !ok { return }
			if progress != tt.wantProgress || speed != tt.wantSpeed || eta != tt.wantETA {
				t.Fatalf("parseProgressLine() = (%v, %q, %q), want (%v, %q, %q)", progress, speed, eta, tt.wantProgress, tt.wantSpeed, tt.wantETA)
			}
		})
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

func TestCancelRejectsCompletedAndUnknownDownloads(t *testing.T) {
	completed := &Download{VideoID: "completed", Status: StatusDone}
	m := &Manager{byID: map[string]*Download{"completed": completed}, order: []string{"completed"}}
	if m.Cancel("completed") { t.Fatal("Cancel() accepted a completed download") }
	if completed.Status != StatusDone { t.Fatalf("completed status changed to %v", completed.Status) }
	if m.Cancel("missing") { t.Fatal("Cancel() accepted an unknown download") }
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


func TestRemoveFinishedQueueEntryPreservesActiveStateAndFiles(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, ".ytui-queue.json")
	completedFile := filepath.Join(dir, "completed.mp4")
	partialFile := filepath.Join(dir, "failed.mp4.part")
	for _, path := range []string{completedFile, partialFile} {
		if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil { t.Fatal(err) }
	}
	completed := &Download{VideoID: "done", Status: StatusDone, FilePath: completedFile}
	queued := &Download{VideoID: "queued", Status: StatusQueued, URL: "https://example.invalid/queued"}
	m := &Manager{
		byID: map[string]*Download{"done": completed, "queued": queued},
		order: []string{"done", "queued"},
		pending: []string{"queued"},
		queueFile: queueFile,
	}
	if m.Remove("queued") { t.Fatal("Remove() accepted a queued download") }
	if !m.Remove("done") { t.Fatal("Remove() rejected a completed download") }
	if got := strings.Join(m.ListIDsForTest(), ","); got != "queued" { t.Fatalf("remaining order = %q, want queued", got) }
	for _, path := range []string{completedFile, partialFile} {
		if _, err := os.Stat(path); err != nil { t.Errorf("queue removal deleted or lost media file %q: %v", path, err) }
	}
	restored := &Manager{byID: make(map[string]*Download), queueFile: queueFile}
	if err := restored.loadQueue(); err != nil { t.Fatalf("loadQueue() error = %v", err) }
	if len(restored.order) != 1 || restored.order[0] != "queued" { t.Fatalf("persisted order = %v, want [queued]", restored.order) }
}

func TestClearCompletedAndFailedQueueEntries(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, ".ytui-queue.json")
	m := &Manager{
		byID: map[string]*Download{
			"done": {VideoID: "done", Status: StatusDone},
			"error": {VideoID: "error", Status: StatusError},
			"resumable": {VideoID: "resumable", Status: StatusResumable},
			"canceled": {VideoID: "canceled", Status: StatusCanceled},
			"queued": {VideoID: "queued", Status: StatusQueued, URL: "https://example.invalid/queued"},
			"paused": {VideoID: "paused", Status: StatusPaused, URL: "https://example.invalid/paused"},
		},
		order: []string{"done", "error", "resumable", "canceled", "queued", "paused"},
		pending: []string{"queued"},
		queueFile: queueFile,
	}
	if got := m.ClearCompleted(); got != 1 { t.Fatalf("ClearCompleted() = %d, want 1", got) }
	if got := m.ClearFailed(); got != 3 { t.Fatalf("ClearFailed() = %d, want 3", got) }
	if got := strings.Join(m.ListIDsForTest(), ","); got != "queued,paused" { t.Fatalf("remaining order = %q, want queued,paused", got) }
	if len(m.pending) != 1 || m.pending[0] != "queued" { t.Fatalf("pending = %v, want [queued]", m.pending) }
	restored := &Manager{byID: make(map[string]*Download), queueFile: queueFile}
	if err := restored.loadQueue(); err != nil { t.Fatalf("loadQueue() error = %v", err) }
	if got := strings.Join(restored.order, ","); got != "queued,paused" { t.Fatalf("persisted order = %q, want queued,paused", got) }
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


func TestManagerShutdownPersistsQueueForRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	destDir := t.TempDir()
	partialPath := filepath.Join(destDir, "First [first-id].mp4.part")
	t.Setenv("YTUI_TEST_PARTIAL_PATH", partialPath)
	script := "#!/bin/sh\n: > \"$YTUI_TEST_PARTIAL_PATH\"\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil { t.Fatal(err) }
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	manager, err := New(destDir)
	if err != nil { t.Fatalf("New() error = %v", err) }
	manager.Enqueue("first-id", "First", "Channel", "https://example.invalid/first", Options{})
	deadline := time.Now().Add(3 * time.Second)
	for {
		items := manager.List()
		if len(items) == 1 && items[0].Status == StatusDownloading { break }
		if time.Now().After(deadline) {
			manager.Shutdown()
			t.Fatalf("first download did not start; queue = %#v", items)
		}
		time.Sleep(10 * time.Millisecond)
	}
	partialDeadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(partialPath); err == nil { break }
		if time.Now().After(partialDeadline) {
			manager.Shutdown()
			t.Fatal("fake yt-dlp did not create its resumable partial file")
		}
		time.Sleep(10 * time.Millisecond)
	}
	manager.Enqueue("second-id", "Second", "Channel", "https://example.invalid/second", Options{})
	manager.Shutdown()

	if _, err := os.Stat(partialPath); err != nil { t.Fatalf("shutdown removed resumable partial file: %v", err) }
	afterShutdown := manager.List()
	if len(afterShutdown) != 2 || afterShutdown[0].Status != StatusQueued || afterShutdown[1].Status != StatusQueued {
		t.Fatalf("shutdown queue = %#v, want both entries queued", afterShutdown)
	}

	restored, err := New(destDir)
	if err != nil { t.Fatalf("restart New() error = %v", err) }
	defer restored.Shutdown()
	restoredItems := restored.List()
	if len(restoredItems) != 2 {
		t.Fatalf("restored queue length = %d, want 2", len(restoredItems))
	}
	if restoredItems[0].VideoID != "first-id" || restoredItems[1].VideoID != "second-id" {
		t.Fatalf("restored queue order = [%s %s], want [first-id second-id]", restoredItems[0].VideoID, restoredItems[1].VideoID)
	}
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

func TestPostProcessDestinationRegex(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "merge", line: `[Merger] Merging formats into "/tmp/video [abc].mp4"`, want: "/tmp/video [abc].mp4"},
		{name: "remux", line: `[VideoRemuxer] Remuxing video from mkv to mp4; Destination: "/tmp/video.mp4"`, want: "/tmp/video.mp4"},
		{name: "audio extraction", line: `[ExtractAudio] Destination: "/tmp/audio.mp3"`, want: "/tmp/audio.mp3"},
		{name: "metadata", line: `[Metadata] Adding metadata to "/tmp/video.mp4"`, want: "/tmp/video.mp4"},
		{name: "thumbnail", line: `[EmbedThumbnail] mutagen: Adding thumbnail to "/tmp/video.mp4"`, want: "/tmp/video.mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := postProcessDestRe.FindStringSubmatch(tt.line)
			if len(match) != 2 || match[1] != tt.want {
				t.Fatalf("postProcessDestRe(%q) = %#v, want path %q", tt.line, match, tt.want)
			}
		})
	}
}

func TestRunTracksFinalOutputAfterMerge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	finalPath := filepath.Join(t.TempDir(), "merged video.mp4")
	t.Setenv("YTUI_TEST_FINAL_PATH", finalPath)
	script := "#!/bin/sh\nprintf '[Merger] Merging formats into \"%s\"\\n' \"$YTUI_TEST_FINAL_PATH\"\n: > \"$YTUI_TEST_FINAL_PATH\"\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := &Download{VideoID: "merge-test", URL: "https://example.invalid/video", Status: StatusQueued}
	m := &Manager{byID: map[string]*Download{d.VideoID: d}, order: []string{d.VideoID}, destDir: t.TempDir()}
	m.run(d)
	if d.Status != StatusDone {
		t.Fatalf("status = %v, want done (error: %v)", d.Status, d.Err)
	}
	if d.FilePath != finalPath {
		t.Fatalf("FilePath = %q, want final merged path %q", d.FilePath, finalPath)
	}
}

func TestRunRejectsMissingFinalOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	missingPath := filepath.Join(t.TempDir(), "missing.mp4")
	t.Setenv("YTUI_TEST_MISSING_PATH", missingPath)
	script := "#!/bin/sh\nprintf '[ExtractAudio] Destination: \"%s\"\\n' \"$YTUI_TEST_MISSING_PATH\"\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := &Download{VideoID: "missing-output-test", URL: "https://example.invalid/video", Status: StatusQueued}
	m := &Manager{byID: map[string]*Download{d.VideoID: d}, order: []string{d.VideoID}, destDir: t.TempDir()}
	m.run(d)
	if d.Status != StatusError {
		t.Fatalf("status = %v, want error for missing output", d.Status)
	}
	if d.Err == nil || !strings.Contains(d.Err.Error(), "final output") {
		t.Fatalf("error = %v, want a missing final output diagnostic", d.Err)
	}
}


func TestResumableQueueEntrySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "Example [video123].mp4.part")
	if err := os.WriteFile(partial, []byte("partial data"), 0o600); err != nil {
		t.Fatal(err)
	}
	queueFile := filepath.Join(dir, ".ytui-queue.json")
	entry := &Download{
		VideoID: "video123", Title: "Example", URL: "https://example.invalid/video",
		Status: StatusResumable, AddedAt: time.Now(),
	}
	manager := &Manager{
		byID: map[string]*Download{"video123": entry},
		order: []string{"video123"},
		queueFile: queueFile,
	}
	manager.mu.Lock()
	err := manager.persistQueueLocked()
	manager.mu.Unlock()
	if err != nil {
		t.Fatalf("persist resumable queue: %v", err)
	}

	restored := &Manager{byID: make(map[string]*Download), queueFile: queueFile}
	if err := restored.loadQueue(); err != nil {
		t.Fatalf("restore queue: %v", err)
	}
	items := restored.List()
	if len(items) != 1 || items[0].Status != StatusResumable {
		t.Fatalf("restored queue = %#v, want one resumable entry", items)
	}
	if len(restored.pending) != 0 {
		t.Fatalf("restored pending queue = %#v, resumable item must wait for explicit retry", restored.pending)
	}
	if data, err := os.ReadFile(partial); err != nil || string(data) != "partial data" {
		t.Fatalf("partial file changed during restore: data=%q err=%v", data, err)
	}
}

func TestPersistQueueRetainsLegacyQueuedAndPausedEntries(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, ".ytui-queue.json")
	legacy := []persistedDownload{
		{VideoID: "queued", URL: "https://example.invalid/queued"},
		{VideoID: "paused", URL: "https://example.invalid/paused", Paused: true},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queueFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	restored := &Manager{byID: make(map[string]*Download), queueFile: queueFile}
	if err := restored.loadQueue(); err != nil {
		t.Fatal(err)
	}
	if got := restored.byID["queued"].Status; got != StatusQueued {
		t.Fatalf("legacy queued status = %v, want queued", got)
	}
	if got := restored.byID["paused"].Status; got != StatusPaused {
		t.Fatalf("legacy paused status = %v, want paused", got)
	}
}


func TestEnqueueCompletedVideoWithChangedOptionsReplacesEntry(t *testing.T) {
	originalOptions := Options{Format: "bestvideo+bestaudio/best", QualityLabel: "Best quality"}
	original := &Download{
		VideoID: "same-video", Title: "Example", URL: "https://example.invalid/video",
		Opts: originalOptions, Status: StatusDone, FilePath: "/downloads/example.mp4",
	}
	manager := &Manager{
		byID: map[string]*Download{"same-video": original},
		order: []string{"same-video"},
		wake: make(chan struct{}, 1),
	}
	duplicate := manager.Enqueue("same-video", "Example", "", original.URL, originalOptions)
	if duplicate != original {
		t.Fatal("identical completed download should not be enqueued again")
	}
	changedOptions := Options{ExtractAudio: true, AudioFormat: "mp3", QualityLabel: "Audio only (mp3)"}
	replacement := manager.Enqueue("same-video", "Example", "", original.URL, changedOptions)
	if replacement == original {
		t.Fatal("changed options did not replace the completed entry")
	}
	if replacement.Status != StatusQueued {
		t.Fatalf("replacement status = %v, want queued", replacement.Status)
	}
	if !replacement.Opts.ForceOverwrite {
		t.Fatal("replacement does not force yt-dlp to regenerate the completed output")
	}
	if len(manager.order) != 1 || manager.order[0] != "same-video" {
		t.Fatalf("queue order = %v, want one entry in its original position", manager.order)
	}
	if len(manager.pending) != 1 || manager.pending[0] != "same-video" {
		t.Fatalf("pending queue = %v, want one replacement entry", manager.pending)
	}
	if replacement.FilePath != "" {
		t.Fatalf("replacement retained old output path %q", replacement.FilePath)
	}
}

func TestSameDownloadOptionsIgnoresInternalOverwriteMarker(t *testing.T) {
	selected := Options{ExtractAudio: true, AudioFormat: "mp3", QualityLabel: "Audio only (mp3)"}
	replacement := selected
	replacement.ForceOverwrite = true
	if !sameDownloadOptions(selected, replacement) {
		t.Fatal("internal overwrite marker should not make selected options different")
	}
}

func TestBuildArgsForReplacementForcesOverwrite(t *testing.T) {
	args := buildArgs(t.TempDir(), &Download{
		URL: "https://example.invalid/video",
		Opts: Options{ForceOverwrite: true},
	})
	found := false
	for _, arg := range args {
		if arg == "--force-overwrites" { found = true; break }
	}
	if !found {
		t.Fatalf("buildArgs() = %#v, want --force-overwrites", args)
	}
}


func TestParseProgressSize(t *testing.T) {
	tests := []struct { line, want string }{
		{"[download] 42.3% of 123MiB at 5.6MiB/s ETA 00:12", "123MiB"},
		{"[download] 12.5% of ~8.00MiB at 1.2MiB/s ETA 00:06", "~8.00MiB"},
		{"[download] 42.3% at 5.6MiB/s ETA 00:12", ""},
	}
	for _, tt := range tests {
		if got := parseProgressSize(tt.line); got != tt.want {
			t.Errorf("parseProgressSize(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestProgressPhaseForYTDLPOutput(t *testing.T) {
	tests := []struct { line, want string }{
		{"[download] Destination: /tmp/video.webm", "Preparing download"},
		{"[download] 42.3% of 123MiB at 5.6MiB/s ETA 00:12", "Downloading"},
		{"[Merger] Merging formats into \"/tmp/video.mp4\"", "Merging formats"},
		{"[VideoRemuxer] Remuxing video from mkv to mp4; Destination: \"/tmp/video.mp4\"", "Remuxing video"},
		{"[AudioRemuxer] Remuxing audio from webm to mp3", "Remuxing audio"},
		{"[ExtractAudio] Destination: \"/tmp/audio.mp3\"", "Extracting audio"},
		{"[Metadata] Adding metadata to \"/tmp/video.mp4\"", "Embedding metadata"},
		{"[EmbedThumbnail] mutagen: Adding thumbnail to \"/tmp/video.mp4\"", "Embedding thumbnail"},
		{"[Fixup] Correcting container in \"/tmp/video.mp4\"", "Finalizing media"},
		{"[debug] unrelated diagnostic", ""},
	}
	for _, tt := range tests {
		if got := progressPhaseForLine(tt.line); got != tt.want {
			t.Errorf("progressPhaseForLine(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}


func TestRunRetainsPostProcessingPhaseOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	outputPath := filepath.Join(t.TempDir(), "failed-merge.mp4")
	t.Setenv("YTUI_TEST_MERGE_PATH", outputPath)
	script := "#!/bin/sh\nprintf '[Merger] Merging formats into \\"%s\\"\\n' " + strconv.Quote(outputPath) + "\nexit 1\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := &Download{VideoID: "merge-failure", URL: "https://example.invalid/video", Status: StatusQueued}
	m := &Manager{byID: map[string]*Download{d.VideoID: d}, order: []string{d.VideoID}, destDir: t.TempDir()}
	m.run(d)
	if d.Status != StatusError {
		t.Fatalf("status = %v, want error", d.Status)
	}
	if d.Phase != "Merging formats" {
		t.Fatalf("phase = %q, want failing merge phase", d.Phase)
	}
	if d.Err == nil || !strings.Contains(d.Err.Error(), "Merging formats into") {
		t.Fatalf("error = %v, want merger diagnostic", d.Err)
	}
}
