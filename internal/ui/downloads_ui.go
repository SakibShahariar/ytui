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
