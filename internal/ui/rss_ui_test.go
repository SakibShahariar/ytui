package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/rss"
	"github.com/SakibShahariar/ytui/internal/store"
)

func TestRSSRefreshShowsItems(t *testing.T) {
	m := Model{store: &store.Store{}}
	updated, _ := m.handleRSSRefresh(rssRefreshMsg{
		items: []rss.Item{{Title: "Channel — Video", Link: "https://www.youtube.com/watch?v=v1"}},
	})
	result := updated.(Model)
	if !result.rssViewer || len(result.rssItems) != 1 {
		t.Fatalf("viewer = %v, items = %d", result.rssViewer, len(result.rssItems))
	}
	if !strings.Contains(result.rssViewerContent(60), "Channel — Video") {
		t.Fatal("item not rendered in RSS viewer")
	}
	closed, _ := result.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if closed.(Model).rssViewer {
		t.Fatal("esc did not close RSS viewer")
	}
}

func TestAutoRSSRespectsInterval(t *testing.T) {
	storeData := &store.Store{RSSFeeds: []store.RSSFeed{{URL: "https://example.com/feed", LastChecked: time.Now().Add(-48 * time.Hour)}}}
	m := Model{store: storeData}
	if cmd := m.maybeAutoRSSCmd(); cmd != nil {
		t.Fatal("automatic refresh scheduled while disabled")
	}
	m.cfg = config.Config{RSSRefreshIntervalHours: 24}
	if cmd := m.maybeAutoRSSCmd(); cmd == nil {
		t.Fatal("due feed did not schedule automatic refresh")
	}
	storeData.RSSFeeds[0].LastChecked = time.Now()
	if cmd := m.maybeAutoRSSCmd(); cmd != nil {
		t.Fatal("freshly checked feed should not trigger a refresh")
	}
}

func TestRSSPaletteWithoutFeeds(t *testing.T) {
	m := Model{store: &store.Store{}}
	updated, _ := m.executePaletteCommand("refresh_rss")
	if !strings.Contains(updated.(Model).statusMsg, "no RSS feeds") {
		t.Fatalf("status = %q", updated.(Model).statusMsg)
	}
}
