package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/thumb"
)

func (m Model) executePaletteCommand(id string) (tea.Model, tea.Cmd) {
	m.commandPalette = false
	m.commandPaletteInput.Blur()
	m.inputMode = false
	m.searchInput.Blur()
	switch id {
	case "search":
		m.activeTab = tabSearch
		m.refreshList()
		m.inputMode = true
		m.searchInput.Focus()
		return m, tea.Batch(textinput.Blink, m.refreshPreview())
	case "home", "refresh_home":
		m.activeTab = tabHome
		if id == "refresh_home" {
			if !m.loggedIn {
				m.statusMsg = "log in with Ctrl+L to refresh Home"
				m.refreshList()
				return m, nil
			}
			m.homeFeed = nil
			m.ytLoading = false
		}
	case "subscriptions":
		m.activeTab = tabSubscriptions
	case "playlists":
		m.activeTab = tabPlaylists
	case "liked":
		m.activeTab = tabLiked
	case "watch_later":
		m.activeTab = tabWatchLater
	case "history":
		m.activeTab = tabHistory
	case "downloads":
		m.activeTab = tabDownloads
		m.offlineDownloads = false
	case "offline":
		m.activeTab = tabDownloads
		m.offlineDownloads = true
	case "toggle_audio":
		m.audioOnly = !m.audioOnly
		if m.audioOnly { m.statusMsg = "audio-only mode ON" } else { m.statusMsg = "audio-only mode OFF" }
		return m, nil
	case "toggle_thumbnails":
		if !m.previewEnabled {
			if !thumb.Supported() {
				m.statusMsg = "this terminal doesn't support Kitty graphics — preview stays off"
				return m, nil
			}
			m.previewEnabled = true
			m.statusMsg = "thumbnail preview ON"
		} else {
			m.previewEnabled = false
			m.statusMsg = "thumbnail preview OFF"
			_ = thumb.Clear()
			m.previewedID = ""
		}
		m.applyListSize()
		return m, m.refreshPreview()
	case "import_shared_queue":
		return m, loadSharedQueueCmd(true)
	case "sponsorblock_settings":
		m = m.openSponsorBlockSettings()
		return m, nil
	case "toggle_autoplay":
		m.cfg.AutoplayNext = !m.cfg.AutoplayNext
		if err := config.Save(m.cfg); err != nil {
			m.errMsg = "save autoplay setting: " + err.Error()
		} else {
			m.errMsg = ""
			if m.cfg.AutoplayNext { m.statusMsg = "autoplay next enabled" } else { m.statusMsg = "autoplay next disabled" }
		}
		return m, nil
	case "clear_filters":
		if m.activeTab != tabHistory && m.activeTab != tabDownloads { m.activeTab = tabHistory }
		m.libraryFilter = ""
		m.libraryFilterInput.SetValue("")
		m.downloadStatusFilter = 0
		m.libraryDateFilter = 0
	case "help":
		m.keyboardHelp = true
		m.keyboardHelpOffset = 0
		return m, nil
	default:
		return m, nil
	}
	m.refreshList()
	return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
}
