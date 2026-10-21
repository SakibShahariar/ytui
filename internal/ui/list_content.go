package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"ytui/internal/downloader"
	"ytui/internal/store"
	"ytui/internal/ytapi"
)

func (m Model) currentListItems() []list.Item {
	switch m.activeTab {
	case tabNowPlaying:
		return nil
	case tabDownloads:
		if m.offlineDownloads {
			items := make([]list.Item, 0, len(m.offlineFiles))
			for _, file := range m.offlineFiles { items = append(items, localDownloadItem{file: file}) }
			return items
		}
		if m.downloads == nil {
			return nil
		}
		dls := m.downloads.List()
		items := make([]list.Item, 0, len(dls))
		for _, d := range dls {
			items = append(items, downloadItem{d})
		}
		return items
	case tabHome:
		return m.homeFeed
	case tabSubscriptions:
		if m.browsingChannelID != "" {
			return m.channelVideos
		}
		if m.loggedIn && len(m.realSubscriptions) > 0 {
			return m.realSubscriptions
		}
		items := make([]list.Item, 0, len(m.store.Subscriptions))
		for _, c := range m.store.Subscriptions {
			items = append(items, subItem{c})
		}
		return items
	case tabPlaylists:
		if m.browsingPlaylistID != "" {
			return m.playlistVideos
		}
		return m.playlists
	case tabLiked:
		return m.likedVideos
	case tabWatchLater:
		items := make([]list.Item, 0, len(m.store.WatchLater))
		for _, e := range m.store.WatchLater {
			items = append(items, watchLaterItem{e})
		}
		return items
	case tabHistory:
		items := make([]list.Item, 0, len(m.store.History))
		for _, e := range m.store.History {
			items = append(items, historyItem{e})
		}
		return items
	default: // tabSearch
		return m.searchResults
	}
}

// maybeFetchForTab kicks off a background fetch if the current tab needs
// YouTube API data that isn't cached yet. Safe to call on every tab
// switch — it no-ops if not logged in, already loading, or already cached.
func (m *Model) maybeFetchForTab() tea.Cmd {
	if !m.loggedIn || m.authClient == nil || m.ytLoading {
		return nil
	}
	switch m.activeTab {
	case tabHome:
		if len(m.homeFeed) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchHomeFeedCmd(m.authClient, nil)
	case tabSubscriptions:
		if m.browsingChannelID != "" {
			if len(m.channelVideos) > 0 {
				return nil
			}
			m.ytLoading = true
			return fetchChannelVideosCmd(m.authClient, m.browsingChannelID)
		}
		if len(m.realSubscriptions) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchSubsCmd(m.authClient)
	case tabPlaylists:
		if m.browsingPlaylistID != "" {
			if len(m.playlistVideos) > 0 {
				return nil
			}
			m.ytLoading = true
			return fetchPlaylistVideosCmd(m.authClient, m.browsingPlaylistID)
		}
		if len(m.playlists) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchPlaylistsCmd(m.authClient)
	case tabLiked:
		if len(m.likedVideos) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchLikedCmd(m.authClient)
	}
	return nil
}

type subItem struct{ c store.Channel }

func (i subItem) Title() string       { return i.c.Name }
func (i subItem) Description() string { return i.c.URL }
func (i subItem) FilterValue() string { return i.c.Name }

type realSubItem struct{ s ytapi.Subscription }

func (i realSubItem) Title() string       { return i.s.Title }
func (i realSubItem) Description() string { return "" }
func (i realSubItem) FilterValue() string { return i.s.Title }

type playlistItemUI struct{ p ytapi.Playlist }

func (i playlistItemUI) Title() string { return i.p.Title }
func (i playlistItemUI) Description() string {
	if i.p.ItemCount == 1 {
		return "1 video"
	}
	return fmt.Sprintf("%d videos", i.p.ItemCount)
}
func (i playlistItemUI) FilterValue() string { return i.p.Title }

type watchLaterItem struct{ e store.WatchLaterEntry }

func (i watchLaterItem) Title() string       { return i.e.Title }
func (i watchLaterItem) Description() string { return i.e.Channel }
func (i watchLaterItem) FilterValue() string { return i.e.Title }

type historyItem struct{ e store.HistoryEntry }

func (i historyItem) Title() string { return i.e.Title }
func (i historyItem) Description() string {
	return fmt.Sprintf("%s · watched %s", i.e.Channel, i.e.WatchedAt.Format("Jan 2 15:04"))
}
func (i historyItem) FilterValue() string { return i.e.Title }

type downloadItem struct{ d downloader.Download }

func (i downloadItem) Title() string { return i.d.Title }
func (i downloadItem) Description() string {
	q := i.d.Opts.QualityLabel
	if q != "" {
		q = q + " · "
	}
	switch i.d.Status {
	case downloader.StatusQueued:
		return q + "queued"
	case downloader.StatusDownloading:
		bar := progressBar(i.d.Progress, 20)
		return fmt.Sprintf("%s%s %5.1f%%  %s  ETA %s", q, bar, i.d.Progress, i.d.Speed, i.d.ETA)
	case downloader.StatusDone:
		return "✓ done — " + i.d.FilePath
	case downloader.StatusError:
		msg := "unknown error"
		if i.d.Err != nil {
			msg = i.d.Err.Error()
		}
		return "✗ error: " + msg
	case downloader.StatusCanceled:
		return "canceled"
	}
	return ""
}
func (i downloadItem) FilterValue() string { return i.d.Title }

// progressBar renders a simple block-character progress bar, e.g.
// "████████░░░░░░░░░░░░" for 40%.
func progressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width) * pct / 100)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

