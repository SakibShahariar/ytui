package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/SakibShahariar/ytui/internal/player"
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
