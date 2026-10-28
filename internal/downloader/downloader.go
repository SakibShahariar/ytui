// Package downloader manages a queue of background yt-dlp downloads with
// live progress, so videos can be saved for offline playback without
// blocking the UI or requiring a separate terminal.
package downloader

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"github.com/SakibShahariar/ytui/internal/notifier"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Status is where a download currently stands.
type Status int

const (
	StatusQueued Status = iota
	StatusDownloading
	StatusDone
	StatusError
	StatusResumable
	StatusCanceled
	StatusPaused
)

// Download tracks one queued/active/finished download. Fields are only
// safe to read while holding the owning Manager's lock — use Manager.List
// for a consistent snapshot rather than reading a *Download directly.
// Options controls how a single download is fetched — the yt-dlp flags
// exposed as actual choices in the UI, not the full universe of yt-dlp's
// flags (most of which aren't meaningful decisions for a casual download).
type Options struct {
	Format             string // yt-dlp -f selector; ignored if ExtractAudio is true
	ExtractAudio       bool   // -x
	AudioFormat        string // --audio-format (e.g. "mp3", "opus"); only used if ExtractAudio
	WriteSubs          bool   // --write-subs --embed-subs
	EmbedThumbnail     bool   // --embed-thumbnail
	CookiesFromBrowser string // --cookies-from-browser <name>; empty = don't use cookies
	QualityLabel       string // display-only, e.g. "1080p" — shown in the Downloads list
}

type Download struct {
	VideoID  string
	Title    string
	Channel  string
	URL      string
	Opts     Options
	Status   Status
	Progress float64 // 0-100
	Speed    string
	ETA      string
	FilePath string
	Err      error

	cmd *exec.Cmd
}

// Manager owns the download queue. Zero value isn't usable — use New().
type Manager struct {
	mu      sync.Mutex
	byID    map[string]*Download
	order   []string // VideoIDs in the order they were enqueued, for stable listing
	destDir string
	closed  bool
	notificationsEnabled bool
	pending []string
	wake chan struct{}
	wg      sync.WaitGroup

	stalePartialsRemoved int
	staleCleanupErr      error
}

// New creates a Manager that saves into destDir (created if it doesn't
// exist). Pass "" to use the default (~/Downloads/ytui).
func New(destDir string) (*Manager, error) {
	if destDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		destDir = filepath.Join(home, "Downloads", "ytui")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("couldn't create download directory: %w", err)
	}
	removed, cleanupErr := CleanupStalePartials(destDir, 30*24*time.Hour)
	m := &Manager{
		byID:                 make(map[string]*Download),
		destDir:              destDir,
		wake:                 make(chan struct{}, 1),
		stalePartialsRemoved: removed,
		staleCleanupErr:      cleanupErr,
	}
	m.wg.Add(1)
	go m.schedule()
	return m, nil
}

// DestDir returns where downloads are being saved.
func (m *Manager) DestDir() string { return m.destDir }

// SetNotificationsEnabled controls best-effort desktop notifications.
func (m *Manager) SetNotificationsEnabled(enabled bool) {
	if m == nil { return }
	m.mu.Lock()
	m.notificationsEnabled = enabled
	m.mu.Unlock()
}

// StaleCleanupResult reports how many old partial artifacts were removed at
// startup and whether any cleanup operation failed. Cleanup errors do not
// prevent the application from starting or downloading.
func (m *Manager) StaleCleanupResult() (int, error) {
	if m == nil {
		return 0, nil
	}
	return m.stalePartialsRemoved, m.staleCleanupErr
}

// Enqueue starts a download in the background. If videoID is already
// queued/downloading/done, this is a no-op (returns the existing entry) —
// no duplicate downloads of the same video.
func (m *Manager) Enqueue(videoID, title, channel, url string, opts Options) *Download {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return &Download{VideoID: videoID, Title: title, Channel: channel, URL: url, Opts: opts, Status: StatusCanceled}
	}
	existing, found := m.byID[videoID]
	if found && existing.Status != StatusError && existing.Status != StatusResumable && existing.Status != StatusCanceled {
		m.mu.Unlock()
		return existing
	}
	d := &Download{VideoID: videoID, Title: title, Channel: channel, URL: url, Opts: opts, Status: StatusQueued}
	m.byID[videoID] = d
	// Retrying a failed/canceled item replaces its entry without adding a
	// second occurrence to the stable display order.
	if !found {
		m.order = append(m.order, videoID)
	}
	m.removePendingLocked(videoID)
	m.pending = append(m.pending, videoID)
	m.mu.Unlock()
	m.signalScheduler()
	return d
}

// List returns a stable-ordered snapshot of all downloads (oldest-enqueued
// first). Safe to call frequently — it copies values out from under the
// lock rather than returning live pointers.
func (m *Manager) List() []Download {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Download, 0, len(m.order))
	for _, id := range m.order {
		if d, ok := m.byID[id]; ok {
			out = append(out, *d) // copy — caller gets a snapshot, not a live pointer
		}
	}
	return out
}

// Active reports whether anything is currently queued or downloading —
// used by the UI to decide whether to keep polling for progress updates.
func (m *Manager) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.order {
		d := m.byID[id]
		if d.Status == StatusQueued || d.Status == StatusDownloading {
			return true
		}
	}
	return false
}

// Cancel kills an in-progress download, if any.
func (m *Manager) Cancel(videoID string) {
	m.mu.Lock()
	d, ok := m.byID[videoID]
	var process *os.Process
	if ok {
		d.Status = StatusCanceled
		// d.cmd is assigned while holding m.mu in run. Snapshot the process
		// under the same lock so cancellation cannot race with process startup.
		if d.cmd != nil {
			process = d.cmd.Process
		}
	}
	m.mu.Unlock()

	// Killing the process can block or trigger worker cleanup; never do it
	// while holding the manager lock.
	if process != nil {
		_ = process.Kill()
	}
}

// Shutdown prevents new downloads, cancels queued and active work, and waits
// for all download workers to finish before the application exits.
func (m *Manager) Shutdown() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.closed = true
	m.pending = nil
	processes := make([]*os.Process, 0)
	for _, d := range m.byID {
		if d.Status != StatusQueued && d.Status != StatusDownloading {
			continue
		}
		d.Status = StatusCanceled
		if d.cmd != nil && d.cmd.Process != nil {
			processes = append(processes, d.cmd.Process)
		}
	}
	m.mu.Unlock()
	m.signalScheduler()

	for _, process := range processes {
		_ = process.Kill()
	}
	m.wg.Wait()
}

const stalePartialAge = 30 * 24 * time.Hour

// CleanupStalePartials removes yt-dlp partial/temp artifacts older than maxAge.
// It only scans the dedicated download directory's immediate children and
// never removes recent files that may still be resumable.
func CleanupStalePartials(destDir string, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, fmt.Errorf("partial-file cleanup age must be positive")
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() || !isPartialArtifact(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		err = os.Remove(filepath.Join(destDir, entry.Name()))
		if err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

func isPartialArtifact(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, ".part") ||
		strings.Contains(name, ".part-frag") ||
		strings.HasSuffix(name, ".ytdl") ||
		strings.HasSuffix(name, ".tmp")
}

func hasPartialForVideo(destDir, videoID string) bool {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return false
	}
	needle := "[" + videoID + "]"
	for _, entry := range entries {
		if !entry.IsDir() && strings.Contains(entry.Name(), needle) && isPartialArtifact(entry.Name()) {
			return true
		}
	}
	return false
}

var progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%.*?at\s+(\S+)\s+ETA\s+(\S+)`)
var destRe = regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`)
var alreadyDoneRe = regexp.MustCompile(`has already been downloaded`)

// buildArgs creates the yt-dlp arguments for a download without starting a
// process. Keeping option translation pure makes the command contract testable
// without requiring yt-dlp to be installed.
func buildArgs(destDir string, d *Download) []string {
	outputTemplate := filepath.Join(destDir, "%(title).100s [%(id)s].%(ext)s")
	args := []string{"--continue", "--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate}

	if d.Opts.ExtractAudio {
		args = append(args, "-x")
		if d.Opts.AudioFormat != "" {
			args = append(args, "--audio-format", d.Opts.AudioFormat)
		}
	} else {
		format := d.Opts.Format
		if format == "" {
			format = "bestvideo+bestaudio/best" // sane default if Options was left zero-valued
		}
		args = append(args, "-f", format)
	}
	if d.Opts.WriteSubs {
		args = append(args, "--write-subs", "--sub-langs", "en.*,en", "--embed-subs")
	}
	if d.Opts.EmbedThumbnail {
		args = append(args, "--embed-thumbnail")
	}
	if d.Opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", d.Opts.CookiesFromBrowser)
	}
	return append(args, d.URL)
}

func (m *Manager) run(d *Download) {
	m.mu.Lock()
	if d.Status != StatusQueued {
		m.mu.Unlock()
		return
	}
	d.Status = StatusDownloading
	m.mu.Unlock()

	args := buildArgs(m.destDir, d)
	cmd := exec.Command("yt-dlp", args...)

	// Use one explicit pipe for both streams. StdoutPipe leaves cmd.Stdout
	// nil, so assigning cmd.Stderr = cmd.Stdout would silently discard stderr.
	stdout, stderrWriter, err := os.Pipe()
	if err != nil {
		m.setError(d.VideoID, fmt.Errorf("couldn't capture yt-dlp output: %w", err))
		return
	}
	cmd.Stdout = stderrWriter
	cmd.Stderr = stderrWriter

	// Serialize process startup with Cancel. Otherwise a cancel between
	// assigning d.cmd and cmd.Start (or before d.cmd is assigned) can leave
	// a canceled queue item running in the background.
	m.mu.Lock()
	if d.Status == StatusCanceled {
		m.mu.Unlock()
		_ = stdout.Close()
		_ = stderrWriter.Close()
		return
	}
	d.cmd = cmd
	startErr := cmd.Start()
	m.mu.Unlock()
	_ = stderrWriter.Close() // the child owns its inherited writer after Start
	if startErr != nil {
		_ = stdout.Close()
		m.setError(d.VideoID, fmt.Errorf("couldn't start yt-dlp: %w", startErr))
		return
	}

	var lastLine string
	reader := bufio.NewReader(stdout)
	for {
		line, oversized, readErr := readOutputLine(reader, 1024*1024)
		if !oversized && line != "" {
			lastLine = line
			if match := progressRe.FindStringSubmatch(line); match != nil {
				pct, _ := strconv.ParseFloat(match[1], 64)
				m.mu.Lock()
				d.Progress = pct
				d.Speed = match[2]
				d.ETA = match[3]
				m.mu.Unlock()
			}
			if match := destRe.FindStringSubmatch(line); match != nil {
				m.mu.Lock()
				d.FilePath = strings.TrimSpace(match[1])
				m.mu.Unlock()
			}
			if alreadyDoneRe.MatchString(line) {
				m.mu.Lock()
				d.Progress = 100
				m.mu.Unlock()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				_ = stdout.Close()
				lastLine = fmt.Sprintf("couldn't read yt-dlp output: %v", readErr)
			}
			break
		}
	}


	err = cmd.Wait()
	resumable := err != nil && hasPartialForVideo(m.destDir, d.VideoID)

	m.mu.Lock()
	if d.Status == StatusCanceled {
		m.mu.Unlock()
		return // Cancel() already set the final status — don't overwrite it
	}
	notifyEnabled := m.notificationsEnabled
	title := d.Title
	if err != nil {
		msg := strings.TrimSpace(lastLine)
		if msg == "" {
			msg = err.Error()
		}
		slog.Error("download failed", "video_id", d.VideoID, "error", msg)
		if resumable {
			d.Status = StatusResumable
			d.Err = fmt.Errorf("%s (partial files retained; press Ctrl+R to resume)", msg)
		} else {
			d.Status = StatusError
			d.Err = fmt.Errorf("%s (press Ctrl+R to retry)", msg)
		}
		m.mu.Unlock()
		if notifyEnabled {
			notifier.Send("Download failed", title)
		}
		return
	}
	d.Status = StatusDone
	d.Progress = 100
	m.mu.Unlock()
	if notifyEnabled {
		notifier.Send("Download complete", title)
	}
}


// readOutputLine drains one complete line from a buffered process stream.
// It retains at most max bytes per line; oversized lines are discarded while
// their remaining bytes are still consumed so the child cannot block on a
// full pipe. The final unterminated line is returned with io.EOF.
func readOutputLine(reader *bufio.Reader, max int) (string, bool, error) {
	var line []byte
	oversized := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if !oversized {
			if len(line)+len(fragment) > max {
				line = nil
				oversized = true
			} else {
				line = append(line, fragment...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return strings.TrimRight(string(line), "\r\n"), oversized, err
	}
}

func (m *Manager) setStatus(videoID string, status Status, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.byID[videoID]; ok {
		if d.Status == StatusCanceled && status != StatusCanceled {
			return
		}
		d.Status = status
		d.Err = err
	}
}

func (m *Manager) setError(videoID string, err error) {
	m.setStatus(videoID, StatusError, err)
}


// signalScheduler wakes the single-download scheduler without blocking callers.
func (m *Manager) signalScheduler() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) removePendingLocked(videoID string) {
	out := m.pending[:0]
	for _, id := range m.pending {
		if id != videoID { out = append(out, id) }
	}
	m.pending = out
}

// schedule runs at most one yt-dlp process at a time, preserving the explicit
// pending order and ensuring queue controls affect actual execution order.
func (m *Manager) schedule() {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		for !m.closed && len(m.pending) == 0 {
			m.mu.Unlock()
			<-m.wake
			m.mu.Lock()
		}
		if m.closed {
			m.mu.Unlock()
			return
		}
		id := m.pending[0]
		m.pending = m.pending[1:]
		d := m.byID[id]
		if d == nil || d.Status != StatusQueued {
			m.mu.Unlock()
			continue
		}
		m.mu.Unlock()
		m.run(d)
	}
}

// MoveQueued moves a pending item one position up (-1) or down (+1).
func (m *Manager) MoveQueued(videoID string, delta int) bool {
	if delta != -1 && delta != 1 { return false }
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed { return false }
	from := -1
	for i, id := range m.pending {
		if id == videoID { from = i; break }
	}
	to := from + delta
	if from < 0 || to < 0 || to >= len(m.pending) { return false }
	m.pending[from], m.pending[to] = m.pending[to], m.pending[from]
	// Keep the visible order consistent with the pending order.
	posA, posB := -1, -1
	for i, id := range m.order {
		if id == videoID { posA = i }
		if id == m.pending[from] { posB = i }
	}
	if posA >= 0 && posB >= 0 { m.order[posA], m.order[posB] = m.order[posB], m.order[posA] }
	return true
}

// Pause suspends a queued download until it is resumed. Active process
// suspension is intentionally not attempted on platforms without a portable
// process-stop API.
func (m *Manager) Pause(videoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[videoID]
	if !ok { return fmt.Errorf("download not found") }
	if d.Status != StatusQueued { return fmt.Errorf("only queued downloads can be paused") }
	m.removePendingLocked(videoID)
	d.Status = StatusPaused
	return nil
}

// Resume puts a paused queued download at the end of the pending queue.
func (m *Manager) Resume(videoID string) error {
	m.mu.Lock()
	d, ok := m.byID[videoID]
	if !ok { m.mu.Unlock(); return fmt.Errorf("download not found") }
	if d.Status != StatusPaused { m.mu.Unlock(); return fmt.Errorf("download is not paused") }
	if m.closed { m.mu.Unlock(); return fmt.Errorf("download manager is shut down") }
	d.Status = StatusQueued
	m.removePendingLocked(videoID)
	m.pending = append(m.pending, videoID)
	m.mu.Unlock()
	m.signalScheduler()
	return nil
}
