package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestHomeRefreshKeepsUnchangedPreview(t *testing.T) {
	v := ytdlp.Video{ID: "same", Title: "Same video", URL: "https://example.com/v"}
	item := videoItem{v: v}
	m := Model{
		activeTab:   tabHome,
		store:       &store.Store{},
		homeFeed:    []list.Item{item},
		previewedID: v.ID,
		searchInput: textinput.New(),
	}
	m.list = list.New([]list.Item{item}, ytDelegate{}, 60, 9)
	updated, _ := m.Update(homeFeedLoadedMsg{videos: []ytdlp.Video{v}, fromCache: false})
	if got := updated.(Model).previewedID; got != v.ID {
		t.Fatalf("unchanged preview was cleared on refresh: %q", got)
	}
}

func TestHomeRefreshReplacesChangedPreview(t *testing.T) {
	old := ytdlp.Video{ID: "old", Title: "Old", URL: "https://example.com/old"}
	newVideo := ytdlp.Video{ID: "new", Title: "New", URL: "https://example.com/new"}
	m := Model{
		activeTab:   tabHome,
		store:       &store.Store{},
		homeFeed:    []list.Item{videoItem{v: old}},
		previewedID: old.ID,
		searchInput: textinput.New(),
	}
	m.list = list.New([]list.Item{videoItem{v: old}}, ytDelegate{}, 60, 9)
	updated, _ := m.Update(homeFeedLoadedMsg{videos: []ytdlp.Video{newVideo}, fromCache: false})
	if got := updated.(Model).previewedID; got != "" {
		t.Fatalf("changed preview was not reset: %q", got)
	}
}
