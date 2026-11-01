package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/downloader"
)

func (m Model) downloadDirectory() string {
	if m.downloads != nil {
		return m.downloads.DestDir()
	}
	if m.cfg.DownloadDir != "" {
		return m.cfg.DownloadDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "ytui")
}

func (m Model) retrySelectedDownload() (tea.Model, tea.Cmd) {
	if m.offlineDownloads {
		return m, nil
	}
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok {
		m.statusMsg = "select a failed or canceled download to retry"
		return m, nil
	}
	switch item.d.Status {
	case downloader.StatusError, downloader.StatusResumable, downloader.StatusCanceled:
	default:
		m.statusMsg = "only failed, resumable, or canceled downloads can be retried"
		return m, nil
	}
	if m.downloads == nil {
		m.statusMsg = "downloads unavailable"
		return m, nil
	}
	wasActive := m.downloads.Active()
	m.downloads.Enqueue(item.d.VideoID, item.d.Title, item.d.Channel, item.d.URL, item.d.Opts)
	m.statusMsg = "retrying/resuming: " + item.d.Title
	if !wasActive {
		return m, downloadsTickCmd()
	}
	return m, nil
}

func (m Model) cancelSelectedDownload() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok {
		m.statusMsg = "select a queued or active download to cancel"
		return m, nil
	}
	if item.d.Status != downloader.StatusQueued && item.d.Status != downloader.StatusDownloading {
		m.statusMsg = "only queued or active downloads can be canceled"
		return m, nil
	}
	m.downloadCancelPending = &downloadCancelRequest{videoID: item.d.VideoID, title: item.d.Title}
	m.statusMsg = "cancel " + item.d.Title + "? press y to confirm, any other key to dismiss"
	return m, nil
}

func (m Model) resolveDownloadCancel(key string, cancel func(string) bool) Model {
	pending := m.downloadCancelPending
	m.downloadCancelPending = nil
	if pending == nil { return m }
	if key != "y" {
		m.statusMsg = "download cancellation dismissed"
		return m
	}
	if cancel == nil {
		m.statusMsg = "downloads unavailable"
		return m
	}
	if !cancel(pending.videoID) {
		m.statusMsg = "download is no longer active"
		return m
	}
	m.statusMsg = "canceled: " + pending.title
	return m
}
func revealDownloadPath(filePath string, open func(string) error) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("download has no recorded file path")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("download file is unavailable: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("download path is a directory, not a file")
	}
	if open == nil {
		return fmt.Errorf("file manager is unavailable")
	}
	return open(filepath.Dir(filePath))
}

func openFileManager(directory string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "explorer", []string{directory}
	case "darwin":
		name, args = "open", []string{directory}
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		name, args = "xdg-open", []string{directory}
	default:
		return fmt.Errorf("opening folders is unsupported on %s", runtime.GOOS)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("file manager command %q is unavailable: %w", name, err)
	}
	command := exec.Command(path, args...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start file manager: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}

func (m Model) revealSelectedDownloadWith(open func(string) error) (Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok {
		m.statusMsg = "select a completed download to reveal"
		return m, nil
	}
	if item.d.Status != downloader.StatusDone {
		m.statusMsg = "only completed downloads can be revealed"
		return m, nil
	}
	if err := revealDownloadPath(item.d.FilePath, open); err != nil {
		m.errMsg = "reveal download: " + err.Error()
		return m, nil
	}
	m.errMsg = ""
	m.statusMsg = "opened folder for " + item.d.Title
	return m, nil
}

func (m Model) revealSelectedDownload() (tea.Model, tea.Cmd) {
	return m.revealSelectedDownloadWith(openFileManager)
}

func (m Model) removeSelectedQueueEntry() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok {
		m.statusMsg = "select a finished queue entry to remove"
		return m, nil
	}
	if !downloaderIsRemovable(item.d.Status) {
		m.statusMsg = "only completed, failed, resumable, or canceled entries can be removed"
		return m, nil
	}
	m.queueRemovePending = &downloadRemoveRequest{videoID: item.d.VideoID, title: item.d.Title}
	m.statusMsg = "remove " + item.d.Title + " from the queue? press y to confirm; media files will be kept"
	return m, nil
}

func downloaderIsRemovable(status downloader.Status) bool {
	return status == downloader.StatusDone || status == downloader.StatusError ||
		status == downloader.StatusResumable || status == downloader.StatusCanceled
}

func (m Model) resolveQueueEntryRemoval(key string, remove func(string) bool) Model {
	pending := m.queueRemovePending
	m.queueRemovePending = nil
	if pending == nil { return m }
	if key != "y" {
		m.statusMsg = "queue entry removal dismissed"
		return m
	}
	if remove == nil {
		m.statusMsg = "downloads unavailable"
		return m
	}
	if !remove(pending.videoID) {
		m.statusMsg = "queue entry is no longer removable"
		return m
	}
	m.errMsg = ""
	m.statusMsg = "removed from queue: " + pending.title
	m.refreshList()
	return m
}

// startPickedDownload enqueues the download with whatever quality/toggle
// choices are currently set in the picker, then closes it.
func (m Model) startPickedDownload() (tea.Model, tea.Cmd) {
	v := m.downloadPickerVideo
	m.downloadPicker = false
	m.downloadPickerVideo = nil
	if v == nil {
		return m, nil
	}
	if m.downloads == nil {
		m.statusMsg = "downloads unavailable — couldn't create ~/Downloads/ytui"
		return m, nil
	}
	q := downloadQualities[m.downloadPickerQuality]
	opts := downloader.Options{
		Format:             q.Format,
		ExtractAudio:       q.ExtractAudio,
		AudioFormat:        q.AudioFormat,
		WriteSubs:          m.downloadPickerSubs,
		SubtitleLang:       m.subtitleLanguage,
		EmbedThumbnail:     m.downloadPickerThumb,
		CookiesFromBrowser: downloadCookieBrowserValues[m.downloadPickerCookies],
		QualityLabel:       q.Label,
	}
	wasActive := m.downloads.Active()
	queued := m.downloads.Enqueue(v.ID, v.Title, v.Channel, v.URL, opts)
	if queued.Opts.ForceOverwrite {
		m.statusMsg = "re-downloading with new options (previous output may be overwritten): " + v.Title
	} else {
		m.statusMsg = "queued (" + q.Label + "): " + v.Title
	}
	if !wasActive {
		return m, downloadsTickCmd() // start the progress-refresh loop
	}
	return m, nil
}


// toggleSelectedDownloadPause pauses or resumes a pending queue entry.
func (m Model) toggleSelectedDownloadPause() (tea.Model, tea.Cmd) {
	if m.offlineDownloads { return m, nil }
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok || m.downloads == nil {
		m.statusMsg = "select a queued download first"
		return m, nil
	}
	var err error
	switch item.d.Status {
	case downloader.StatusQueued, downloader.StatusDownloading:
		err = m.downloads.Pause(item.d.VideoID)
	case downloader.StatusPaused:
		err = m.downloads.Resume(item.d.VideoID)
	default:
		m.statusMsg = "only queued or active downloads can be paused or resumed"
		return m, nil
	}
	if err != nil {
		m.errMsg = "change download queue state: " + err.Error()
		return m, nil
	}
	m.errMsg = ""
	if item.d.Status == downloader.StatusQueued || item.d.Status == downloader.StatusDownloading {
		m.statusMsg = "paused: " + item.d.Title
	} else {
		m.statusMsg = "resumed: " + item.d.Title
	}
	m.refreshList()
	if m.downloads.Active() { return m, downloadsTickCmd() }
	return m, nil
}

// moveSelectedDownload reorders a pending download and refreshes the visible queue.
func (m Model) moveSelectedDownload(delta int) (tea.Model, tea.Cmd) {
	if m.offlineDownloads { return m, nil }
	item, ok := m.list.SelectedItem().(downloadItem)
	if !ok || m.downloads == nil {
		m.statusMsg = "select a queued download first"
		return m, nil
	}
	if !m.downloads.MoveQueued(item.d.VideoID, delta) {
		m.statusMsg = "download cannot move further in the pending queue"
		return m, nil
	}
	m.statusMsg = "download queue order updated"
	m.refreshList()
	return m, nil
}
