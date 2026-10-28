package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/store"
)

func (m Model) subscribeToSelected() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok || v.Channel == "" {
		return m, nil
	}
	if v.ChannelID == "" && v.ChannelURL == "" {
		m.statusMsg = "can't subscribe: channel identifier unavailable in search metadata"
		return m, nil
	}
	id := v.ChannelID
	if id == "" {
		id = v.ChannelURL
	}
	if err := m.store.Subscribe(store.Channel{ID: id, Name: v.Channel, URL: v.ChannelURL}); err != nil {
		m.statusMsg = "failed to subscribe: " + err.Error()
		return m, nil
	}
	m.statusMsg = "subscribed to " + v.Channel
	return m, nil
}

func (m Model) addSelectedToWatchLater() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	_ = m.store.AddWatchLater(store.WatchLaterEntry{VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL, Thumbnail: v.Thumbnail, DurationS: v.Duration})
	m.statusMsg = "added to watch later: " + v.Title
	return m, nil
}
