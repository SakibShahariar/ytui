package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func resumeTestModel(t *testing.T, position, duration float64) Model {
	t.Helper()
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	video := ytdlp.Video{ID: "video-1", Title: "A video", URL: "https://example.com/video", Duration: duration}
	if err := s.AddHistory(store.HistoryEntry{
		VideoID: video.ID, Title: video.Title, URL: video.URL, PositionS: position,
	}); err != nil {
		t.Fatal(err)
	}
	l := list.New([]list.Item{videoItem{v: video}}, ytDelegate{}, defaultWidth, defaultHeight)
	l.Select(0)
	return Model{store: s, list: l, activeTab: tabSearch, width: defaultWidth, height: defaultHeight}
}

func TestPlaySelectedOffersResumeAndCanStartOver(t *testing.T) {
	m := resumeTestModel(t, 125, 600)
	updated, cmd := m.playSelected()
	got := updated.(Model)
	if cmd != nil {
		t.Fatal("playSelected() started playback before a choice")
	}
	if got.resumePrompt == nil || got.resumePrompt.position != 125 {
		t.Fatalf("resume prompt = %#v, want saved position 125", got.resumePrompt)
	}
	if got.playingVid != nil {
		t.Fatal("video should not be marked as playing before the choice")
	}

	updated, cmd = got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("starting from beginning should return a playback command")
	}
	if got.resumePrompt != nil || got.playingVid == nil {
		t.Fatal("start-over choice did not transition into playback")
	}
	if got.store.History[0].PositionS != 0 {
		t.Fatalf("history position = %v, want reset to 0", got.store.History[0].PositionS)
	}
}

func TestPlaySelectedIgnoresStaleResumePosition(t *testing.T) {
	m := resumeTestModel(t, 900, 300)
	updated, cmd := m.playSelected()
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("stale position should fall back to starting from beginning")
	}
	if got.resumePrompt != nil {
		t.Fatal("stale position should not prompt for resume")
	}
	if got.store.History[0].PositionS != 0 {
		t.Fatalf("history position = %v, want reset to 0", got.store.History[0].PositionS)
	}
}

func TestResumePromptEnterResumesAtSavedPosition(t *testing.T) {
	m := resumeTestModel(t, 125, 600)
	updated, _ := m.playSelected()
	prompted := updated.(Model)
	updated, cmd := prompted.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("resume choice should return a playback command")
	}
	if got.resumePrompt != nil || got.playingVid == nil {
		t.Fatal("resume choice did not transition into playback")
	}
	if got.store.History[0].PositionS != 125 {
		t.Fatalf("history position = %v, want preserved resume position 125", got.store.History[0].PositionS)
	}
}
