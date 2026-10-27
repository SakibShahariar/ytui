package ui

import (
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
