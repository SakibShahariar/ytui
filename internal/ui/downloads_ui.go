package ui

import (
	"os"
	"path/filepath"

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
	if item, ok := m.list.SelectedItem().(downloadItem); ok {
		if m.downloads != nil && (item.d.Status == downloader.StatusQueued || item.d.Status == downloader.StatusDownloading) {
			m.downloads.Cancel(item.d.VideoID)
			m.statusMsg = "canceled: " + item.d.Title
		}
	}
	return m, nil
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
	m.downloads.Enqueue(v.ID, v.Title, v.Channel, v.URL, opts)
	m.statusMsg = "queued (" + q.Label + "): " + v.Title
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
