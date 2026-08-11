// Package downloader manages a queue of background yt-dlp downloads with
// live progress, so videos can be saved for offline playback without
// blocking the UI or requiring a separate terminal.
package downloader

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Status is where a download currently stands.
type Status int

const (
	StatusQueued Status = iota
	StatusDownloading
	StatusDone
	StatusError
	StatusCanceled
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
	return &Manager{
		byID:    make(map[string]*Download),
		destDir: destDir,
	}, nil
}

// DestDir returns where downloads are being saved.
func (m *Manager) DestDir() string { return m.destDir }

// Enqueue starts a download in the background. If videoID is already
// queued/downloading/done, this is a no-op (returns the existing entry) —
// no duplicate downloads of the same video.
func (m *Manager) Enqueue(videoID, title, channel, url string, opts Options) *Download {
	m.mu.Lock()
	if existing, ok := m.byID[videoID]; ok && existing.Status != StatusError && existing.Status != StatusCanceled {
		m.mu.Unlock()
		return existing
	}
	d := &Download{VideoID: videoID, Title: title, Channel: channel, URL: url, Opts: opts, Status: StatusQueued}
	m.byID[videoID] = d
	m.order = append(m.order, videoID)
	m.mu.Unlock()

	go m.run(d)
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
	if ok {
		d.Status = StatusCanceled
	}
	m.mu.Unlock()
	if !ok || d.cmd == nil || d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Kill()
}

var progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%.*?at\s+(\S+)\s+ETA\s+(\S+)`)
var destRe = regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`)
var alreadyDoneRe = regexp.MustCompile(`has already been downloaded`)

func (m *Manager) run(d *Download) {
	m.setStatus(d.VideoID, StatusDownloading, nil)

	outputTemplate := filepath.Join(m.destDir, "%(title).100s [%(id)s].%(ext)s")
	args := []string{"--newline", "--no-warnings", "--no-playlist", "-o", outputTemplate}

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
	args = append(args, d.URL)

	cmd := exec.Command("yt-dlp", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.setError(d.VideoID, err)
		return
	}
	cmd.Stderr = cmd.Stdout // merge — yt-dlp's progress line format is the same either way

	m.mu.Lock()
	d.cmd = cmd
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		m.setError(d.VideoID, fmt.Errorf("couldn't start yt-dlp: %w", err))
		return
	}

	var lastLine string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
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

	err = cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()
	if d.Status == StatusCanceled {
		return // Cancel() already set the final status — don't overwrite it
	}
	if err != nil {
		d.Status = StatusError
		msg := strings.TrimSpace(lastLine)
		if msg == "" {
			msg = err.Error()
		}
		d.Err = fmt.Errorf("%s", msg)
		return
	}
	d.Status = StatusDone
	d.Progress = 100
}

func (m *Manager) setStatus(videoID string, status Status, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.byID[videoID]; ok {
		d.Status = status
		d.Err = err
	}
}

func (m *Manager) setError(videoID string, err error) {
	m.setStatus(videoID, StatusError, err)
}
