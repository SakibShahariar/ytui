package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/thumb"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applyListSize()
		_ = thumb.Clear()
		m.previewedID = ""
		// Bubbletea normally only redraws what changed, which can leave
		// stale characters behind at columns/rows a new, narrower frame no
		// longer writes to — a full clear on every resize avoids that.
		cmds := []tea.Cmd{tea.ClearScreen}
		if m.previewVisible() {
			if v, ok := m.selectedVideo(); ok {
				cmds = append(cmds, m.showThumbnailCmd(v))
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case directURLMsg:
		m.directURL = ""
		if msg.err != nil {
			m.errMsg = "open URL: " + msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.playingLocal = false
		m.playingVid = &msg.video
		m.statusMsg = "starting mpv: " + msg.video.Title
		_ = m.store.AddHistory(store.HistoryEntry{
			VideoID: msg.video.ID, Title: msg.video.Title, Channel: msg.video.Channel,
			URL: msg.video.URL, Thumbnail: msg.video.Thumbnail,
		})
		return m, startPlayback(msg.video.URL, m.audioOnly, 0)

	case searchResultsMsg:
		m.loading = false
		m.everSearched = true
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		items := make([]list.Item, 0, len(msg.results))
		for _, v := range msg.results {
			items = append(items, videoItem{v})
		}
		m.searchResults = items
		if m.listPositions == nil {
			m.listPositions = make(map[tab]int)
		}
		// A new query is a new result set; start at the top rather than
		// restoring the index from the previous search.
		m.listPositions[tabSearch] = 0
		m.list.SetItems(items)
		m.list.Select(0)
		m.listTab = tabSearch
		m.listTabReady = true
		m.applyListSize()
		m.statusMsg = fmt.Sprintf("%d results", len(items))
		var cmd tea.Cmd
		if len(items) > 0 {
			if vi, ok := items[0].(videoItem); ok {
				cmd = m.showThumbnailCmd(vi.v)
			}
		}
		return m, cmd

	case playbackStartedMsg:
		m.nowPlaying = msg.p
		m.npPos, m.npDur, m.npVol, m.npPaused = 0, 0, 0, false
		m.npDescription = ""
		if msg.p != nil && msg.err != nil {
			// mpv started but its IPC socket was unavailable. Keep the
			// process represented as Now Playing until it actually exits.
			m.errMsg = "mpv is playing (no IPC control: pause/seek/volume unavailable)"
			return m, tea.Batch(tea.ClearScreen, waitForPlaybackExitCmd(msg.p))
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.errMsg = ""
		}
		if msg.err == nil && msg.p != nil {
			cmds := []tea.Cmd{tea.ClearScreen, positionTickCmd(), nowPlayingTickCmd()}
			if m.playingVid != nil {
				m.npDescriptionLoading = true
				cmds = append(cmds, fetchDescriptionCmd(m.playingVid.ID, m.playingVid.URL))
			}
			if m.activeTab == tabNowPlaying {
				cmds = append(cmds, m.refreshPreview())
			}
			return m, tea.Batch(cmds...)
		}
		return m, tea.ClearScreen

	case playbackExitedMsg:
		if msg.p == m.nowPlaying {
			m.saveResumePosition()
			return m.clearNowPlaying()
		}
		return m, nil

	case videoDescriptionMsg:
		m.npDescriptionLoading = false
		if msg.err != nil || m.playingVid == nil || m.playingVid.ID != msg.videoID {
			return m, nil // stale response for a video that's no longer playing, or fetch failed — not worth surfacing as an error
		}
		m.npDescription = msg.desc
		return m, nil

	case nowPlayingTickMsg:
		if m.nowPlaying == nil {
			return m, nil // playback ended/stopped since the last tick — don't reschedule
		}
		pos, err := m.nowPlaying.Position()
		if err != nil {
			// mpv is gone — the video finished, or it was closed some way
			// other than our own ctrl+x (its own quit key, closing its
			// window, a crash). Save the last UI-observed position before
			// clearing playback state; the fresh IPC query has already failed.
			m.saveResumePosition()
			return m.clearNowPlaying()
	}
		m.npPos = pos
		if dur, err := m.nowPlaying.Duration(); err == nil {
			m.npDur = dur
		}
		if vol, err := m.nowPlaying.Volume(); err == nil {
			m.npVol = vol
		}
		if paused, err := m.nowPlaying.IsPaused(); err == nil {
			m.npPaused = paused
		}
		return m, nowPlayingTickCmd()

	case positionTickMsg:
		if m.nowPlaying == nil {
			return m, nil // playback ended/stopped since the last tick — don't reschedule
		}
		m.saveResumePosition()
		return m, positionTickCmd()

	case downloadsTickMsg:
		if m.activeTab == tabDownloads {
			m.refreshList()
		}
		if m.downloads != nil && m.downloads.Active() {
			return m, downloadsTickCmd()
		}
		return m, nil

	case thumbShownMsg:
		m.previewedID = msg.id
		return m, nil
	case thumbErrMsg:
		m.errMsg = "thumbnail: " + msg.err.Error()
		return m, nil

	case loginResultMsg:
		if msg.err != nil {
			m.errMsg = "login: " + msg.err.Error()
			m.statusMsg = ""
			return m, nil
		}
		m.statusMsg = "verifying login…"
		return m, authClientCmd()

	case clientSecretsImportedMsg:
		m.credentialStep = 0
		m.pendingClientID = ""
		m.pathInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.showLoginHelp = false
		m.statusMsg = "client_secret.json imported ✓ — opening browser to log in…"
		return m, loginCmd()

	case authClientReadyMsg:
		if msg.err != nil {
			m.errMsg = "login: " + msg.err.Error()
			m.loggedIn = false
			m.authClient = nil
			return m, nil
		}
		m.loggedIn = true
		m.authClient = msg.client
		m.errMsg = ""
		return m, m.maybeFetchForTab()

	case subsLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "subscriptions: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.subs))
		for _, s := range msg.subs {
			items = append(items, realSubItem{s})
		}
		m.realSubscriptions = items
		if m.activeTab == tabSubscriptions {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case playlistsLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "playlists: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.playlists))
		for _, p := range msg.playlists {
			items = append(items, playlistItemUI{p})
		}
		m.playlists = items
		if m.activeTab == tabPlaylists && m.browsingPlaylistID == "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case playlistVideosLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "playlist videos: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.playlistVideos = items
		if m.activeTab == tabPlaylists && m.browsingPlaylistID != "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case channelVideosLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "channel uploads: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.channelVideos = items
		if m.activeTab == tabSubscriptions && m.browsingChannelID != "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case homeFeedLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			if len(m.homeFeed) > 0 {
				m.errMsg = "home feed refresh failed (showing cached results): " + homeFeedErrorMessage(msg.err)
			} else {
				m.errMsg = "home feed: " + homeFeedErrorMessage(msg.err)
			}
			return m, nil
		}
		if msg.failed > 0 {
			m.statusMsg = fmt.Sprintf("home feed loaded with partial results: %d channel(s) failed", msg.failed)
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.homeFeed = items
		if msg.fromCache {
			m.statusMsg = fmt.Sprintf("showing cached Home feed (%s old)", msg.cacheAge.Round(time.Minute))
			if msg.refresh {
				m.statusMsg += "; refreshing in background…"
				m.ytLoading = true
			}
		} else if msg.failed == 0 {
			m.statusMsg = "Home feed refreshed"
		}
		if m.activeTab == tabHome {
			m.refreshList()
			previewCmd := m.refreshPreview()
			if msg.fromCache && msg.refresh {
				return m, tea.Batch(previewCmd, fetchHomeFeedNetworkCmd(m.authClient, msg.subscriptions))
			}
			return m, previewCmd
		}
		if msg.fromCache && msg.refresh {
			return m, fetchHomeFeedNetworkCmd(m.authClient, msg.subscriptions)
		}
		return m, nil

	case likedLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "liked videos: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.likedVideos = items
		if m.activeTab == tabLiked {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	return m, nil
}

