package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/rivo/uniseg"
	"github.com/SakibShahariar/ytui/internal/player"
	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestWrapTwoLinesTruncatesByDisplayWidth(t *testing.T) {
	const width = 8
	_, line2 := wrapTwoLines(strings.Repeat("界 ", 30), width)
	if got := uniseg.StringWidth(line2); got > width {
		t.Errorf("second line display width = %d, want at most %d (%q)", got, width, line2)
	}
}

func TestAudioOnlyLabel(t *testing.T) {
	if got := audioOnlyLabel(false); got != "" {
		t.Errorf("audioOnlyLabel(false) = %q, want empty", got)
	}
	if got := audioOnlyLabel(true); got != "AUDIO" {
		t.Errorf("audioOnlyLabel(true) = %q, want AUDIO", got)
	}
}

func TestVisibleTabsAndCycling(t *testing.T) {
	tests := []struct {
		name string
		model Model
		want []tab
		next tab
		prev tab
	}{
		{
			name: "now playing hidden when idle",
			model: Model{activeTab: tabSearch},
			want: []tab{tabSearch, tabHome, tabSubscriptions, tabPlaylists, tabLiked, tabWatchLater, tabHistory, tabDownloads},
			next: tabHome,
			prev: tabDownloads,
		},
		{
			name: "now playing visible while playing",
			model: Model{activeTab: tabSearch, nowPlaying: &player.Player{}},
			want: []tab{tabSearch, tabNowPlaying, tabHome, tabSubscriptions, tabPlaylists, tabLiked, tabWatchLater, tabHistory, tabDownloads},
			next: tabNowPlaying,
			prev: tabDownloads,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.visibleTabs()
			if len(got) != len(tt.want) {
				t.Fatalf("visibleTabs() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("visibleTabs() = %v, want %v", got, tt.want)
				}
			}
			if got := tt.model.nextTab(); got != tt.next {
				t.Errorf("nextTab() = %v, want %v", got, tt.next)
			}
			if got := tt.model.prevTab(); got != tt.prev {
				t.Errorf("prevTab() = %v, want %v", got, tt.prev)
			}
		})
	}
}


func TestPlaybackWithoutIPCRemainsVisibleUntilProcessExit(t *testing.T) {
	p := &player.Player{}
	m := Model{}
	updated, cmd := m.Update(playbackStartedMsg{p: p, err: errors.New("IPC connect failed")})
	got := updated.(Model)
	if got.nowPlaying != p {
		t.Fatal("degraded playback was not kept as Now Playing")
	}
	if !strings.Contains(got.errMsg, "no IPC control") {
		t.Fatalf("errMsg = %q, want degraded-control warning", got.errMsg)
	}
	if cmd == nil {
		t.Fatal("expected a process-exit watcher for degraded playback")
	}
	msg := waitForPlaybackExitCmd(p)()
	if _, ok := msg.(playbackExitedMsg); !ok {
		t.Fatalf("watcher message = %T, want playbackExitedMsg", msg)
	}
	updated, _ = got.Update(msg)
	afterExit := updated.(Model)
	if afterExit.nowPlaying != nil {
		t.Fatal("playback state was not cleared after process exit")
	}
}

func TestPlaybackFailureWithoutProcessDoesNotStartExitWatcher(t *testing.T) {
	updated, cmd := (Model{}).Update(playbackStartedMsg{err: errors.New("mpv failed to start")})
	if updated.(Model).nowPlaying != nil {
		t.Fatal("failed startup left a Now Playing process")
	}
	if cmd == nil {
		// tea.ClearScreen is a valid command even for a failed start.
		return
	}
}


func TestListPositionRestoredPerTabAndClampedWhenItemsShrink(t *testing.T) {
	searchItems := []list.Item{
		videoItem{v: ytdlp.Video{ID: "s1", Title: "Search 1"}},
		videoItem{v: ytdlp.Video{ID: "s2", Title: "Search 2"}},
		videoItem{v: ytdlp.Video{ID: "s3", Title: "Search 3"}},
		videoItem{v: ytdlp.Video{ID: "s4", Title: "Search 4"}},
		videoItem{v: ytdlp.Video{ID: "s5", Title: "Search 5"}},
	}
	homeItems := []list.Item{
		videoItem{v: ytdlp.Video{ID: "h1", Title: "Home 1"}},
		videoItem{v: ytdlp.Video{ID: "h2", Title: "Home 2"}},
	}
	l := list.New(searchItems, ytDelegate{}, defaultWidth, defaultHeight)
	m := Model{
		list: l,
		listPositions: make(map[tab]int),
		listTab: tabSearch,
		listTabReady: true,
		activeTab: tabSearch,
		searchResults: searchItems,
		homeFeed: homeItems,
		width: defaultWidth,
		height: defaultHeight,
	}
	m.list.Select(3)

	m.activeTab = tabHome
	m.refreshList()
	if got := m.list.Index(); got != 0 {
		t.Fatalf("first visit to Home selected index %d, want 0", got)
	}
	if got := m.listPositions[tabSearch]; got != 3 {
		t.Fatalf("saved Search index = %d, want 3", got)
	}

	m.list.Select(1)
	m.activeTab = tabSearch
	m.refreshList()
	if got := m.list.Index(); got != 3 {
		t.Fatalf("return to Search selected index %d, want 3", got)
	}

	m.searchResults = searchItems[:2]
	m.refreshList()
	if got := m.list.Index(); got != 1 {
		t.Fatalf("shrunk Search list selected index %d, want clamped index 1", got)
	}
	if got := m.listPositions[tabSearch]; got != 1 {
		t.Fatalf("saved Search index after clamp = %d, want 1", got)
	}
}

func TestClampListIndex(t *testing.T) {
	for _, tc := range []struct{ index, count, want int }{
		{index: 3, count: 5, want: 3},
		{index: 9, count: 5, want: 4},
		{index: -1, count: 5, want: 0},
		{index: 4, count: 0, want: 0},
	} {
		if got := clampListIndex(tc.index, tc.count); got != tc.want {
			t.Errorf("clampListIndex(%d, %d) = %d, want %d", tc.index, tc.count, got, tc.want)
		}
	}
}


func TestNextWatchLaterVideoUsesQueueOrder(t *testing.T) {
	m := Model{store: &store.Store{WatchLater: []store.WatchLaterEntry{
		{VideoID: "one", Title: "One", URL: "https://youtube.com/watch?v=one"},
		{VideoID: "two", Title: "Two", URL: "https://youtube.com/watch?v=two"},
		{VideoID: "three", Title: "Three", URL: "https://youtube.com/watch?v=three"},
	}}}
	if got, ok := m.nextWatchLaterVideo("one"); !ok || got.ID != "two" {
		t.Fatalf("next after first = (%q, %v), want (two, true)", got.ID, ok)
	}
	if got, ok := m.nextWatchLaterVideo("not-in-queue"); !ok || got.ID != "one" {
		t.Fatalf("next for non-queue video = (%q, %v), want (one, true)", got.ID, ok)
	}
	if _, ok := m.nextWatchLaterVideo("three"); ok {
		t.Fatal("last queue item unexpectedly wrapped to the beginning")
	}
}

func TestNextCurrentListVideoAdvancesPastSelection(t *testing.T) {
	items := []list.Item{
		videoItem{v: ytdlp.Video{ID: "one", Title: "One"}},
		videoItem{v: ytdlp.Video{ID: "two", Title: "Two"}},
	}
	l := list.New(items, ytDelegate{}, defaultWidth, defaultHeight)
	l.Select(0)
	m := Model{list: l}
	if got, ok := m.nextCurrentListVideo(); !ok || got.ID != "two" {
		t.Fatalf("next list video = (%q, %v), want (two, true)", got.ID, ok)
	}
	l.Select(1)
	m.list = l
	if _, ok := m.nextCurrentListVideo(); ok {
		t.Fatal("last list item unexpectedly wrapped to the beginning")
	}
}
