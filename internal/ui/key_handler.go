package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/auth"
	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/library"
	"github.com/SakibShahariar/ytui/internal/thumb"
)

// handleKey is the single source of truth for keyboard routing.
//
// There's no manual mode toggle to think about: on the Search tab, typing
// any letter/digit automatically starts (or continues) editing the query;
// pressing an arrow key or Enter-to-play automatically hands control back
// to list navigation. Because plain letters can now appear inside a typed
// query at any moment, all the old single-letter shortcuts (s/w/p/a/t/x/q)
// live on ctrl+ combos instead, so they never collide with what you're
// typing and work identically on every tab.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.subtitlePicker {
		switch key {
		case "esc":
			m.subtitlePicker = false
			m.subtitlePickerVideo = nil
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.subtitlePickerCursor > 0 { m.subtitlePickerCursor-- }
			return m, nil
		case "down":
			if m.subtitlePickerCursor < len(m.subtitleTracks) { m.subtitlePickerCursor++ }
			return m, nil
		case "enter":
			if m.subtitlePickerCursor == 0 {
				m.subtitleLanguage = ""
				m.statusMsg = "subtitles disabled"
			} else {
				track := m.subtitleTracks[m.subtitlePickerCursor-1]
				m.subtitleLanguage = track.Language
				m.statusMsg = "subtitle language selected: " + track.Name + " (" + track.Language + ")"
			}
			m.subtitlePicker = false
			m.subtitlePickerVideo = nil
			return m, nil
		default:
			return m, nil
		}
	}

	if m.commandPalette {
		switch key {
		case "esc":
			m.commandPalette = false
			m.commandPaletteInput.Blur()
			m.statusMsg = "command palette closed"
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.commandPaletteCursor > 0 { m.commandPaletteCursor-- }
			return m, nil
		case "down":
			commands := m.filteredPaletteCommands()
			if m.commandPaletteCursor < len(commands)-1 { m.commandPaletteCursor++ }
			return m, nil
		case "pgup":
			m.commandPaletteCursor -= 5
			if m.commandPaletteCursor < 0 { m.commandPaletteCursor = 0 }
			return m, nil
		case "pgdown":
			commands := m.filteredPaletteCommands()
			m.commandPaletteCursor += 5
			if m.commandPaletteCursor >= len(commands) { m.commandPaletteCursor = len(commands)-1 }
			return m, nil
		case "home":
			m.commandPaletteCursor = 0
			return m, nil
		case "end":
			m.commandPaletteCursor = len(m.filteredPaletteCommands())-1
			if m.commandPaletteCursor < 0 { m.commandPaletteCursor = 0 }
			return m, nil
		case "enter":
			commands := m.filteredPaletteCommands()
			if len(commands) == 0 { m.statusMsg = "no matching commands"; return m, nil }
			if m.commandPaletteCursor >= len(commands) { m.commandPaletteCursor = len(commands)-1 }
			return m.executePaletteCommand(commands[m.commandPaletteCursor].id)
		case "ctrl+u":
			m.commandPaletteInput.SetValue("")
			m.commandPaletteCursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.commandPaletteInput, cmd = m.commandPaletteInput.Update(msg)
			m.commandPaletteCursor = 0
			return m, cmd
		}
	}

	if m.watchLaterBulkPending {
		m.watchLaterBulkPending = false
		switch key {
		case "y":
			ids := make([]string, 0, len(m.watchLaterSelected))
			for id, selected := range m.watchLaterSelected { if selected { ids = append(ids, id) } }
			if err := m.store.RemoveWatchLaterMany(ids); err != nil {
				m.errMsg = "remove selected Watch Later items: " + err.Error()
			} else {
				m.errMsg = ""
				m.watchLaterSelected = make(map[string]bool)
				m.statusMsg = fmt.Sprintf("removed %d Watch Later item(s)", len(ids))
				m.refreshList()
			}
			return m, nil
		case "n", "esc":
			m.statusMsg = "bulk removal canceled"
			return m, nil
		default:
			m.statusMsg = "bulk removal canceled"
			return m, nil
		}
	}

	if m.watchLaterClearPending {
		m.watchLaterClearPending = false
		switch key {
		case "y":
			if err := m.store.ClearWatchLater(); err != nil {
				m.errMsg = "clear Watch Later: " + err.Error()
			} else {
				m.errMsg = ""
				m.statusMsg = "Watch Later queue cleared"
				m.watchLaterSelected = make(map[string]bool)
				m.refreshList()
			}
			return m, nil
		case "n", "esc":
			m.statusMsg = "clear Watch Later canceled"
			return m, nil
		default:
			m.statusMsg = "clear Watch Later canceled"
			return m, nil
		}
	}

	if m.offlineDeletePending != nil {
		pending := m.offlineDeletePending
		m.offlineDeletePending = nil
		switch key {
		case "y":
			if err := library.Delete(m.downloadDirectory(), *pending); err != nil {
				m.errMsg = "delete download: " + err.Error()
			} else {
				m.errMsg = ""
				m.statusMsg = "deleted " + pending.Name
			}
			m.refreshList()
			return m, nil
		case "n", "esc":
			m.statusMsg = "delete canceled"
			return m, nil
		default:
			m.statusMsg = "delete canceled"
			return m, nil
		}
	}

	if key == "ctrl+c" {
		// The application-level cleanup hook owns player, download, and
		// terminal-image cleanup for every exit path, not just this key.
		return m, tea.Quit
	}

	if m.libraryFilterMode {
		switch key {
		case "enter":
			m.libraryFilter = strings.TrimSpace(m.libraryFilterInput.Value())
			m.libraryFilterMode = false
			m.libraryFilterInput.Blur()
			m.refreshList()
			m.statusMsg = "library filter applied"
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		case "esc":
			m.libraryFilterMode = false
			m.libraryFilterInput.Blur()
			m.libraryFilterInput.SetValue(m.libraryFilter)
			m.applyListSize()
			return m, nil
		case "ctrl+u":
			m.libraryFilterInput.SetValue("")
			return m, nil
		default:
			var cmd tea.Cmd
			m.libraryFilterInput, cmd = m.libraryFilterInput.Update(msg)
			return m, cmd
		}
	}

	if m.resumePrompt != nil {
		prompt := *m.resumePrompt
		switch key {
		case "enter", "r":
			return m.startSelectedVideo(prompt.video, prompt.position)
		case "b":
			return m.startSelectedVideo(prompt.video, 0)
		case "esc":
			m.resumePrompt = nil
			m.statusMsg = "playback canceled"
			return m, nil
		default:
			return m, nil
		}
	}

	if m.keyboardHelp {
		switch key {
		case "esc", "?":
			m.keyboardHelp = false
			m.keyboardHelpOffset = 0
			return m, nil
		case "up":
			if m.keyboardHelpOffset > 0 { m.keyboardHelpOffset-- }
			return m, nil
		case "down":
			if m.keyboardHelpOffset < m.keyboardHelpMaxOffset() { m.keyboardHelpOffset++ }
			return m, nil
		case "pgup":
			m.keyboardHelpOffset -= 8
			if m.keyboardHelpOffset < 0 { m.keyboardHelpOffset = 0 }
			return m, nil
		case "pgdown":
			m.keyboardHelpOffset += 8
			if m.keyboardHelpOffset > m.keyboardHelpMaxOffset() { m.keyboardHelpOffset = m.keyboardHelpMaxOffset() }
			return m, nil
		case "home":
			m.keyboardHelpOffset = 0
			return m, nil
		case "end":
			m.keyboardHelpOffset = len(keyboardHelpLines())
			return m, nil
		default:
			return m, nil
		}
	}

	if key == "?" && !m.inputMode && m.credentialStep == 0 && !m.downloadPicker && !m.showLoginHelp && m.offlineDeletePending == nil {
		m.keyboardHelp = true
		m.keyboardHelpOffset = 0
		return m, nil
	}

	if m.downloadPicker {
		switch key {
		case "up":
			if m.downloadPickerCursor > 0 {
				m.downloadPickerCursor--
			}
			return m, nil
		case "down":
			if m.downloadPickerCursor < downloadPickerRows()-1 {
				m.downloadPickerCursor++
			}
			return m, nil
		case "enter", " ":
			qCount := len(downloadQualities)
			switch {
			case m.downloadPickerCursor < qCount:
				m.downloadPickerQuality = m.downloadPickerCursor
			case m.downloadPickerCursor == qCount:
				m.downloadPickerSubs = !m.downloadPickerSubs
			case m.downloadPickerCursor == qCount+1:
				m.downloadPickerThumb = !m.downloadPickerThumb
			case m.downloadPickerCursor == qCount+2:
				m.downloadPickerCookies = (m.downloadPickerCookies + 1) % len(downloadCookieBrowserValues)
			default: // "Start Download" row
				return m.startPickedDownload()
			}
			return m, nil
		case "esc":
			m.downloadPicker = false
			m.downloadPickerVideo = nil
			return m, nil
		}
		return m, nil // swallow anything else while the picker is open
	}

	if m.credentialStep > 0 {
		switch key {
		case "enter":
			raw := strings.TrimSpace(m.pathInput.Value())
			if raw == "" {
				return m, nil
			}
			if m.credentialStep == 1 {
				if strings.HasPrefix(raw, "{") {
					// Power-user shortcut: they pasted a whole downloaded
					// client_secret.json instead of just the Client ID.
					m.credentialStep = 0
					m.pathInput.Blur()
					m.pathInput.SetValue("")
					m.statusMsg = "importing…"
					return m, importClientSecretsRawCmd(raw)
				}
				m.pendingClientID = raw
				m.credentialStep = 2
				m.pathInput.SetValue("")
				m.pathInput.Placeholder = "paste your Client Secret…"
				return m, nil
			}
			// credentialStep == 2: raw is the Client Secret.
			clientID := m.pendingClientID
			m.credentialStep = 0
			m.pendingClientID = ""
			m.pathInput.Blur()
			m.pathInput.SetValue("")
			m.statusMsg = "importing…"
			return m, importClientCredentialsCmd(clientID, raw)
		case "esc":
			m.credentialStep = 0
			m.pendingClientID = ""
			m.pathInput.Blur()
			m.pathInput.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}

	if key == ":" && !m.inputMode && !m.showLoginHelp && m.credentialStep == 0 {
		m.commandPalette = true
		m.commandPaletteCursor = 0
		m.commandPaletteInput.SetValue("")
		m.commandPaletteInput.Focus()
		return m, tea.Batch(tea.ClearScreen, textinput.Blink)
	}

	if m.activeTab == tabWatchLater && !m.inputMode {
		switch key {
		case " ":
			item, ok := m.list.SelectedItem().(watchLaterItem)
			if !ok { return m, nil }
			if m.watchLaterSelected == nil { m.watchLaterSelected = make(map[string]bool) }
			if m.watchLaterSelected[item.e.VideoID] { delete(m.watchLaterSelected, item.e.VideoID) } else { m.watchLaterSelected[item.e.VideoID] = true }
			m.refreshList()
			if len(m.watchLaterSelected) == 0 { m.statusMsg = "Watch Later selection cleared" } else { m.statusMsg = fmt.Sprintf("%d Watch Later item(s) selected", len(m.watchLaterSelected)) }
			return m, nil
		case "m":
			if len(m.watchLaterSelected) == 0 { m.statusMsg = "select items with Space before bulk removal"; return m, nil }
			m.watchLaterBulkPending = true
			m.statusMsg = fmt.Sprintf("remove %d selected Watch Later item(s)? press y to confirm, any other key to cancel", len(m.watchLaterSelected))
			return m, nil
		case "s":
			m.watchLaterSortMode = (m.watchLaterSortMode + 1) % 4
			modes := []string{"added", "title", "duration", "channel"}
			labels := []string{"oldest added", "title A–Z", "shortest duration", "channel A–Z"}
			if err := m.store.SortWatchLater(modes[m.watchLaterSortMode]); err != nil {
				m.errMsg = "sort Watch Later: " + err.Error()
				return m, nil
			}
			m.errMsg = ""
			m.statusMsg = "Watch Later sorted by " + labels[m.watchLaterSortMode]
			m.refreshList()
			return m, nil
		case "x":
			if len(m.store.WatchLater) == 0 {
				m.statusMsg = "Watch Later queue is already empty"
				return m, nil
			}
			m.watchLaterClearPending = true
			m.statusMsg = fmt.Sprintf("remove all %d Watch Later items? press y to confirm, any other key to cancel", len(m.store.WatchLater))
			return m, nil
		}
	}

	switch key {
	case "tab":
		m.searchInput.Blur()
		m.inputMode = false
		m.activeTab = m.nextTab()
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
	case "shift+tab":
		m.searchInput.Blur()
		m.inputMode = false
		m.activeTab = m.prevTab()
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())

	case "/":
		if !m.inputMode {
			m.activeTab = tabSearch
			m.refreshList()
			m.inputMode = true
			m.searchInput.Focus()
			return m, tea.Batch(tea.ClearScreen, textinput.Blink, m.refreshPreview())
		}

	case "ctrl+l":
		if m.loggedIn {
			_ = auth.Logout()
			m.loggedIn = false
			m.authClient = nil
			m.realSubscriptions = nil
			m.playlists = nil
			m.playlistVideos = nil
			m.browsingPlaylistID = ""
			m.channelVideos = nil
			m.browsingChannelID = ""
			m.likedVideos = nil
			m.homeFeed = nil
			m.refreshList()
			m.statusMsg = "logged out"
			return m, nil
		}
		if !auth.HaveClientSecrets() {
			m.showLoginHelp = !m.showLoginHelp
			return m, nil
		}
		m.showLoginHelp = false
		m.statusMsg = "opening browser to log in with Google…"
		m.errMsg = ""
		return m, loginCmd()

	case "up", "down", "pgup", "pgdown", "home", "end":
		// Arrow-family keys always mean "navigate the list" — release the
		// search box automatically if it was focused, no Esc needed.
		if m.inputMode {
			m.inputMode = false
			m.searchInput.Blur()
		}
		return m.moveSelection(msg)

	case "enter":
		if m.activeTab == tabDownloads && m.offlineDownloads {
			if item, ok := m.list.SelectedItem().(localDownloadItem); ok {
				return m.playLocalFile(item.file)
			}
			return m, nil
		}
		if m.inputMode {
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				return m, nil
			}
			m.loading = true
			m.inputMode = false
			m.searchInput.Blur()
			return m, doSearch(query)
		}
		if m.showLoginHelp && !auth.HaveClientSecrets() {
			m.credentialStep = 1
			m.pathInput.Placeholder = "paste your Client ID…"
			m.pathInput.Focus()
			m.pathInput.SetValue("")
			m.errMsg = ""
			return m, textinput.Blink
		}
		if m.activeTab == tabPlaylists && m.browsingPlaylistID == "" {
			if pl, ok := m.list.SelectedItem().(playlistItemUI); ok {
				m.browsingPlaylistID = pl.p.ID
				m.browsingPlaylistTitle = pl.p.Title
				m.playlistVideos = nil
				m.refreshList()
				return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
			}
			return m, nil
		}
		if m.activeTab == tabSubscriptions && m.browsingChannelID == "" {
			if s, ok := m.list.SelectedItem().(realSubItem); ok {
				m.browsingChannelID = s.s.ChannelID
				m.browsingChannelTitle = s.s.Title
				m.channelVideos = nil
				m.refreshList()
				return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
			}
			// Local (non-API) subscriptions don't carry a real channel ID,
			// so there's nothing to browse into — Enter is a no-op there
			// rather than a broken drill-in.
			return m, nil
		}
		return m.playSelected()

	case "esc":
		if m.inputMode {
			m.inputMode = false
			m.searchInput.Blur()
			return m, nil
		}
		if m.showLoginHelp {
			m.showLoginHelp = false
			return m, nil
		}
		if m.activeTab == tabPlaylists && m.browsingPlaylistID != "" {
			m.browsingPlaylistID = ""
			m.playlistVideos = nil
			m.refreshList()
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		}
		if m.activeTab == tabSubscriptions && m.browsingChannelID != "" {
			m.browsingChannelID = ""
			m.channelVideos = nil
			m.refreshList()
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		}
		return m, nil

	case "ctrl+f":
		if m.activeTab == tabHistory || m.activeTab == tabDownloads {
			m.libraryFilterMode = true
			m.libraryFilterInput.SetValue(m.libraryFilter)
			m.libraryFilterInput.Focus()
			m.applyListSize()
			return m, tea.Batch(tea.ClearScreen, textinput.Blink)
		}
		return m, nil

	case "ctrl+u":
		if m.activeTab == tabHistory || m.activeTab == tabDownloads {
			m.libraryFilter = ""
			m.libraryFilterInput.SetValue("")
			m.downloadStatusFilter = 0
			m.libraryDateFilter = 0
			m.refreshList()
			m.statusMsg = "library filters cleared"
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		}
		return m, nil

	case "ctrl+space":
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			return m.toggleSelectedDownloadPause()
		}
		return m, nil

	case "ctrl+up":
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			return m.moveSelectedDownload(-1)
		}
		return m, nil

	case "ctrl+down":
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			return m.moveSelectedDownload(1)
		}
		return m, nil

	case "ctrl+y":
		if m.activeTab == tabHistory || m.activeTab == tabDownloads {
			m.libraryDateFilter = (m.libraryDateFilter + 1) % 4
			m.refreshList()
			m.statusMsg = "date filter: " + m.libraryDateFilterLabel()
			return m, nil
		}
		return m, nil

	case "ctrl+g":
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			m.downloadStatusFilter = (m.downloadStatusFilter + 1) % 4
			m.refreshList()
			m.statusMsg = "download status filter: " + m.downloadStatusFilterLabel()
			return m, nil
		}
		return m, nil

	case "ctrl+t":
		m.previewEnabled = !m.previewEnabled
		if m.previewEnabled {
			if thumb.Supported() {
				m.statusMsg = "thumbnail preview ON (experimental — ctrl+t again if the screen glitches)"
			} else {
				m.previewEnabled = false
				m.statusMsg = "this terminal doesn't support Kitty graphics — preview stays off"
			}
		} else {
			m.statusMsg = "thumbnail preview OFF"
			_ = thumb.Clear()
			m.previewedID = ""
		}
		m.applyListSize()
		return m, nil

	case "ctrl+a":
		m.audioOnly = !m.audioOnly
		if m.audioOnly {
			m.statusMsg = "audio-only mode ON"
		} else {
			m.statusMsg = "audio-only mode OFF"
		}
		return m, nil

	case "ctrl+s":
		return m.subscribeToSelected()
	case "ctrl+v":
		if v, ok := m.selectedVideo(); ok {
			m.errMsg = ""
			m.statusMsg = "checking available subtitles…"
			return m, loadSubtitleTracksCmd(v)
		}
		m.statusMsg = "select a video to inspect subtitle tracks"
		return m, nil

	case "ctrl+w":
		return m.addSelectedToWatchLater()
	case "ctrl+o":
		if m.activeTab == tabDownloads {
			m.offlineDownloads = !m.offlineDownloads
			m.errMsg = ""
			m.refreshList()
			if m.offlineDownloads {
				m.statusMsg = fmt.Sprintf("offline downloads: %d media file(s)", len(m.offlineFiles))
			} else {
				m.statusMsg = "download queue"
			}
		}
		return m, nil

	case "delete":
		if m.activeTab == tabDownloads && m.offlineDownloads {
			if item, ok := m.list.SelectedItem().(localDownloadItem); ok {
				file := item.file
				m.offlineDeletePending = &file
				m.statusMsg = "delete " + file.Name + "? press y to confirm, any other key to cancel"
			}
		}
		return m, nil

	case "ctrl+r":
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			return m.retrySelectedDownload()
		}
		return m, nil

	case "ctrl+d":
		if m.activeTab == tabDownloads {
			if m.offlineDownloads { return m, nil }
			return m.cancelSelectedDownload()
		}
		if v, ok := m.selectedVideo(); ok {
			m.downloadPicker = true
			m.downloadPickerVideo = &v
			m.downloadPickerCursor = 0
			m.downloadPickerQuality = defaultQualityIndex(m.cfg.DefaultQuality)
			m.downloadPickerSubs = false
			m.downloadPickerThumb = false
			m.downloadPickerCookies = m.defaultCookieBrowserIndex()
		}
		return m, nil

	case "ctrl+n":
		m.cfg.AutoplayNext = !m.cfg.AutoplayNext
		if err := config.Save(m.cfg); err != nil {
			m.errMsg = "save autoplay setting: " + err.Error()
		} else {
			m.errMsg = ""
			if m.cfg.AutoplayNext { m.statusMsg = "autoplay next enabled" } else { m.statusMsg = "autoplay next disabled" }
		}
		return m, nil

	case "ctrl+p":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.TogglePause()
		}
		return m, nil
	case "[":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.SeekRelative(-10)
		}
		return m, nil
	case "]":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.SeekRelative(10)
		}
		return m, nil
	case "-":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.VolumeAdjust(-5)
		}
		return m, nil
	case "=":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.VolumeAdjust(5)
		}
		return m, nil
	case "ctrl+x":
		if m.nowPlaying != nil {
			m.saveResumePosition()
			_ = m.nowPlaying.Quit()
			m.statusMsg = "playback stopped"
			return m.clearNowPlaying()
		}
		return m, nil
	}

	// Already typing — everything else (letters, digits, backspace,
	// left/right cursor movement, punctuation) goes to the text box.
	if m.inputMode {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	// Not typing yet: a plain letter/digit/symbol on the Search tab means
	// "start a query" — auto-focus the box and forward this keystroke as
	// its first character, instead of requiring an explicit "/" first.
	if m.activeTab == tabSearch && msg.Type == tea.KeyRunes {
		m.inputMode = true
		m.searchInput.Focus()
		m.searchInput.SetValue("")
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, tea.Batch(cmd, textinput.Blink)
	}

	// Everything else — j/k, g/G, etc. — goes to the list. On the Search
	// tab, letters are claimed by typing above, so only non-letter keys
	// (already handled explicitly, e.g. arrows) reach here; on the other
	// tabs, where there's no text box to worry about, this restores full
	// list-widget navigation (j/k included).
	return m.moveSelection(msg)
}

