package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/sponsorblock"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestSponsorBlockSettingsDefaultCategoriesAndSave(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := Model{searchInput: textinput.New()}
	m = m.openSponsorBlockSettings()
	if !containsSponsorCategory(m.sponsorBlockDraftCategories, "sponsor") || !containsSponsorCategory(m.sponsorBlockDraftCategories, "selfpromo") {
		t.Fatalf("first-run categories = %#v, want sponsor and selfpromo", m.sponsorBlockDraftCategories)
	}
	m.sponsorBlockDraftEnabled = true
	m.sponsorBlockCursor = 1
	m = m.toggleSponsorBlockSetting() // turn off sponsor; leave selfpromo enabled
	updated, cmd := m.saveSponsorBlockSettings()
	got := updated.(Model)
	if cmd != nil { t.Fatal("saving settings unexpectedly started an async command without active playback") }
	if !got.cfg.SponsorBlockEnabled || !got.cfg.SponsorBlockCategoriesConfigured {
		t.Fatalf("settings not saved: %#v", got.cfg)
	}
	if len(got.cfg.SponsorBlockCategories) != 1 || got.cfg.SponsorBlockCategories[0] != "selfpromo" {
		t.Fatalf("skip categories = %#v, want [selfpromo]", got.cfg.SponsorBlockCategories)
	}
	saved, err := config.LoadWithError()
	if err != nil { t.Fatal(err) }
	if !saved.SponsorBlockEnabled || len(saved.SponsorBlockCategories) != 1 || saved.SponsorBlockCategories[0] != "selfpromo" {
		t.Fatalf("persisted SponsorBlock settings = %#v", saved)
	}
}

func TestSponsorBlockViewerCanCancelSettings(t *testing.T) {
	m := Model{sponsorBlockSettings: true, sponsorBlockDraftEnabled: true, searchInput: textinput.New()}
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.sponsorBlockSettings || !got.sponsorBlockDraftEnabled { t.Fatalf("Escape changed draft unexpectedly: %#v", got) }
	if cmd != nil { t.Fatal("Escape returned unexpected command") }
}

func TestSponsorBlockAutoSkipsOnlySelectedCategories(t *testing.T) {
	session := &fakePlaybackSession{position: 12}
	m := Model{
		nowPlaying: session,
		playingVid: &ytdlp.Video{ID: "video"},
		cfg: config.Config{SponsorBlockEnabled: true, SponsorBlockCategoriesConfigured: true, SponsorBlockCategories: []string{"sponsor"}},
		sponsorSegmentVideoID: "video",
		sponsorSegments: []sponsorblock.Segment{
			{Start: 10, End: 20, Category: "sponsor", ActionType: "skip", UUID: "s1"},
		},
	}
	updated, _ := m.Update(nowPlayingTickMsg{})
	got := updated.(Model)
	if !session.seekCalled || session.seekTo != 20 { t.Fatalf("seek called=%v target=%v; want seek to 20", session.seekCalled, session.seekTo) }
	if got.sponsorLastSkipped != "s1" || got.npPos != 20 || !strings.Contains(got.statusMsg, "auto-skipped sponsor") {
		t.Fatalf("auto-skip state = %#v", got)
	}
}

func TestSponsorBlockHighlightAndManualSkip(t *testing.T) {
	session := &fakePlaybackSession{}
	m := Model{
		nowPlaying: session,
		playingVid: &ytdlp.Video{ID: "video"},
		npPos: 12,
		cfg: config.Config{SponsorBlockEnabled: true},
		sponsorSegmentVideoID: "video",
		sponsorSegments: []sponsorblock.Segment{{Start: 10, End: 20, Category: "sponsor", ActionType: "skip", UUID: "s1"}},
	}
	if got := m.nextSponsorHighlight(); !strings.Contains(got, "SB NOW: sponsor") { t.Fatalf("highlight = %q", got) }
	updated, _ := m.skipCurrentSponsorSegment()
	got := updated.(Model)
	if !session.seekCalled || session.seekTo != 20 { t.Fatalf("manual skip seek called=%v target=%v", session.seekCalled, session.seekTo) }
	if !strings.Contains(got.statusMsg, "manually skipped sponsor") { t.Fatalf("status = %q", got.statusMsg) }
}
