package ui

import (
	"net/http"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/ytapi"
)

func fetchSubsCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		subs, err := ytapi.Subscriptions(client)
		return subsLoadedMsg{subs: subs, err: err}
	}
}

func fetchPlaylistsCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		pls, err := ytapi.Playlists(client)
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

func fetchPlaylistVideosCmd(client *http.Client, playlistID string) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.PlaylistVideos(client, playlistID)
		return playlistVideosLoadedMsg{videos: videos, err: err}
	}
}

func fetchChannelVideosCmd(client *http.Client, channelID string) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.ChannelVideos(client, channelID)
		return channelVideosLoadedMsg{videos: videos, err: err}
	}
}

func fetchHomeFeedCmd(client *http.Client, subs []ytapi.Subscription) tea.Cmd {
	return func() tea.Msg {
		cached, age, cacheErr := loadHomeFeedCache()
		if cacheErr == nil && len(cached) > 0 {
			return homeFeedLoadedMsg{
				videos: cached, fromCache: true, cacheAge: age,
				refresh: age >= homeFeedCacheTTL, subscriptions: subs,
			}
		}
		return fetchHomeFeedNetworkCmd(client, subs)()
	}
}

func fetchHomeFeedNetworkCmd(client *http.Client, subs []ytapi.Subscription) tea.Cmd {
	return func() tea.Msg {
		if len(subs) == 0 {
			var err error
			subs, err = ytapi.Subscriptions(client)
			if err != nil {
				return homeFeedLoadedMsg{err: err}
			}
		}
		if len(subs) == 0 {
			return homeFeedLoadedMsg{videos: nil}
		}
		videos, failed, err := ytapi.HomeFeed(client, subs, 5)
		if err == nil && len(videos) > 0 {
			_ = saveHomeFeedCache(videos)
		}
		return homeFeedLoadedMsg{videos: videos, failed: failed, err: err}
	}
}

func fetchLikedCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.LikedVideos(client)
		return likedLoadedMsg{videos: videos, err: err}
	}
}

