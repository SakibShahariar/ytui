package ui

import (
	"strconv"
	"strings"

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
	case "reveal_download":
		m.activeTab = tabDownloads
		m.offlineDownloads = false
		m.refreshList()
		return m.revealSelectedDownload()
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
	case "sponsorblock_stats":
		m.sponsorBlockStats = true
		return m, nil
	case "settings":
		return m.openSettings()
	case "check_updates":
		m.statusMsg = "checking for updates…"
		return m, checkUpdatesCmd()
	case "manage_channels":
		return m.openChannelManager()
	case "refresh_rss":
		if m.store == nil || len(m.store.RSSFeeds) == 0 {
			m.statusMsg = "no RSS feeds subscribed (use 'ytui rss add URL')"
			return m, nil
		}
		m.statusMsg = "refreshing RSS feeds…"
		return m, refreshRSSCmd(m.store)
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
		if strings.HasPrefix(id, "theme:") {
			name := strings.TrimPrefix(id, "theme:")
			// Selecting a built-in theme is a request for that theme's pure
			// palette. Custom role overrides belong to a separate profile and
			// are cleared so "default" really looks like the default.
			hadOverrides := len(m.cfg.Colors) > 0
			if err := applyTheme(name, nil); err != nil {
				m.errMsg = "theme: " + err.Error()
				return m, nil
			}
			m.restyleComponents()
			m.cfg.Theme = name
			m.cfg.Colors = nil
			if err := config.SetTheme(name, nil); err != nil {
				m.errMsg = "save theme: " + err.Error()
			} else if hadOverrides {
				m.statusMsg = "theme set to " + name + " (custom colors cleared)"
			} else {
				m.statusMsg = "theme set to " + name
			}
			return m, tea.ClearScreen
		}
		if !strings.HasPrefix(id, "preset:") {
			return m, nil
		}
		index, err := strconv.Atoi(strings.TrimPrefix(id, "preset:"))
		if err != nil || index < 0 || index >= len(m.downloadPresets()) {
			return m, nil
		}
		video, ok := m.selectedVideo()
		if !ok {
			m.statusMsg = "select a video first to apply a preset"
			return m, nil
		}
		m.downloadPicker = true
		m.downloadPickerVideo = &video
		m.downloadPickerCursor = 0
		m.downloadPickerQuality = 0
		m.downloadPickerSubs = false
		m.downloadPickerThumb = false
		m.downloadPickerCookies = m.defaultCookieBrowserIndex()
		m.downloadPickerRate = 0
		m.downloadPickerRateValue = ""
		m.downloadPickerPreset = index
		m.applyDownloadPreset(index)
		return m, nil
	}
	m.refreshList()
	return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
}
