package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestSponsorBlockAPIFailureDoesNotInterruptPlayback(t *testing.T) {
	session := &fakePlaybackSession{position: 5}
	m := Model{
		cfg: config.Config{SponsorBlockEnabled: true},
		playingVid: &ytdlp.Video{ID: "current-video"},
		nowPlaying: session,
		sponsorSegments: nil,
	}
	updated, cmd := m.Update(sponsorSegmentsMsg{videoID: "current-video", err: errors.New("network unavailable")})
	got := updated.(Model)
	if got.nowPlaying != session { t.Fatal("SponsorBlock failure stopped current playback") }
	if !strings.Contains(got.statusMsg, "playback continues") { t.Fatalf("status = %q", got.statusMsg) }
	if cmd != nil { t.Fatal("API failure unexpectedly returned a playback command") }
}

func TestSponsorBlockIgnoresStaleSegmentResponse(t *testing.T) {
	session := &fakePlaybackSession{}
	m := Model{
		cfg: config.Config{SponsorBlockEnabled: true},
		playingVid: &ytdlp.Video{ID: "new-video"},
		nowPlaying: session,
		sponsorSegmentVideoID: "new-video",
	}
	updated, _ := m.Update(sponsorSegmentsMsg{videoID: "old-video"})
	got := updated.(Model)
	if got.sponsorSegmentVideoID != "new-video" { t.Fatalf("segment video ID changed to %q", got.sponsorSegmentVideoID) }
}
