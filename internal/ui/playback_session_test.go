package ui

import (
	"errors"
	"testing"

	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
	tea "github.com/charmbracelet/bubbletea"
)

func TestResumePositionNeedsSaveThresholdAndForcedEvents(t *testing.T) {
	tests := []struct {
		name string
		previous float64
		next float64
		force bool
		want bool
	}{
		{name: "below threshold", previous: 10, next: 19.9, want: false},
		{name: "at threshold", previous: 10, next: 20, want: true},
		{name: "backward seek below threshold", previous: 80, next: 70.1, want: false},
		{name: "backward seek at threshold", previous: 80, next: 70, want: true},
		{name: "first position below threshold", previous: 0, next: 9.9, want: false},
		{name: "first position at threshold", previous: 0, next: 10, want: true},
		{name: "forced pause or exit", previous: 20, next: 21, force: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resumePositionNeedsSave(tt.previous, tt.next, tt.force); got != tt.want {
				t.Fatalf("resumePositionNeedsSave(%v, %v, %v) = %v, want %v", tt.previous, tt.next, tt.force, got, tt.want)
			}
		})
	}
}

type fakePlaybackSession struct {
	waitCalled bool
	position float64
	seekTo     float64
	seekCalled bool
}

func (f *fakePlaybackSession) Position() (float64, error) { return f.position, nil }
func (f *fakePlaybackSession) Duration() (float64, error) { return 0, nil }
func (f *fakePlaybackSession) Volume() (float64, error) { return 0, nil }
func (f *fakePlaybackSession) IsPaused() (bool, error) { return false, nil }
func (f *fakePlaybackSession) Quit() error { return nil }
func (f *fakePlaybackSession) TogglePause() error { return nil }
func (f *fakePlaybackSession) SeekRelative(int) error { return nil }
func (f *fakePlaybackSession) SeekAbsolute(position float64) error {
	f.seekTo = position
	f.seekCalled = true
	return nil
}
func (f *fakePlaybackSession) VolumeAdjust(int) error { return nil }
func (f *fakePlaybackSession) Wait() error {
	f.waitCalled = true
	return nil
}

func TestPlaybackSessionCanDriveUIWithoutMpv(t *testing.T) {
	session := &fakePlaybackSession{}
	updated, cmd := (Model{}).Update(playbackStartedMsg{p: session})
	got := updated.(Model)
	if got.nowPlaying != session {
		t.Fatal("fake playback session was not installed as Now Playing")
	}
	if cmd == nil {
		t.Fatal("expected playback UI commands")
	}

	msg := waitForPlaybackExitCmd(session)()
	if !session.waitCalled {
		t.Fatal("exit watcher did not wait on the injected session")
	}
	exited, ok := msg.(playbackExitedMsg)
	if !ok {
		t.Fatalf("watcher message = %T, want playbackExitedMsg", msg)
	}
	if exited.p != session {
		t.Fatal("exit watcher returned a different playback session")
	}

	updated, _ = got.Update(msg)
	if updated.(Model).nowPlaying != nil {
		t.Fatal("UI did not clear the fake playback session after exit")
	}
}

func TestStartPlaybackWithInjectsLauncherAndResumes(t *testing.T) {
	session := &fakePlaybackSession{}
	called := false

	msg := startPlaybackWith("https://example.com/video", true, 42.5, func(url string, audioOnly bool) (playbackSession, error) {
		called = true
		if url != "https://example.com/video" {
			t.Fatalf("URL = %q, want original URL", url)
		}
		if !audioOnly {
			t.Fatal("audio-only option was not forwarded")
		}
		return session, nil
	})()

	got, ok := msg.(playbackStartedMsg)
	if !ok {
		t.Fatalf("message type = %T, want playbackStartedMsg", msg)
	}
	if !called || got.p != session || got.err != nil {
		t.Fatalf("playback result = %#v, launcher called = %v", got, called)
	}
	if !session.seekCalled || session.seekTo != 42.5 {
		t.Fatalf("resume seek = (%v, %v), want (true, 42.5)", session.seekCalled, session.seekTo)
	}
}

func TestStartPlaybackWithPreservesStartupFailure(t *testing.T) {
	wantErr := errors.New("mpv unavailable")
	msg := startPlaybackWith("https://example.com/video", false, 0, func(string, bool) (playbackSession, error) {
		return nil, wantErr
	})()

	got, ok := msg.(playbackStartedMsg)
	if !ok {
		t.Fatalf("message type = %T, want playbackStartedMsg", msg)
	}
	if got.p != nil || !errors.Is(got.err, wantErr) {
		t.Fatalf("startup result = %#v, want nil session and %v", got, wantErr)
	}
}

func TestPlaybackStartedWithUnavailableIPCRemainsTracked(t *testing.T) {
	session := &fakePlaybackSession{}
	startErr := errors.New("mpv started without IPC control")

	updated, cmd := (Model{}).Update(playbackStartedMsg{p: session, err: startErr})
	got := updated.(Model)
	if got.nowPlaying != session {
		t.Fatal("degraded playback session was not retained as Now Playing")
	}
	if got.errMsg != "mpv is playing (no IPC control: pause/seek/volume unavailable)" {
		t.Fatalf("error message = %q, want degraded-IPC explanation", got.errMsg)
	}
	if cmd == nil {
		t.Fatal("expected exit watcher for degraded playback session")
	}
}

func TestStalePlaybackExitDoesNotClearNewSession(t *testing.T) {
	oldSession := &fakePlaybackSession{}
	currentSession := &fakePlaybackSession{}
	model := Model{nowPlaying: currentSession}

	updated, cmd := model.Update(playbackExitedMsg{p: oldSession})
	got := updated.(Model)
	if got.nowPlaying != currentSession {
		t.Fatal("stale exit message cleared the newer playback session")
	}
	if cmd != nil {
		t.Fatal("stale exit message unexpectedly scheduled follow-up work")
	}
}

func TestPositionTickStopsWhenNoPlaybackSessionExists(t *testing.T) {
	updated, cmd := (Model{}).Update(positionTickMsg{})
	if cmd != nil {
		t.Fatal("position tick scheduled another tick without active playback")
	}
	if updated.(Model).nowPlaying != nil {
		t.Fatal("position tick unexpectedly created a playback session")
	}
}


func TestPausePersistsResumePositionImmediately(t *testing.T) {
	t.Setenv("YTUI_DATA_PATH", t.TempDir()+"/data.json")
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := saved.AddHistory(store.HistoryEntry{
		VideoID: "video-1", Title: "Example", URL: "https://example.test/video", PositionS: 10,
	}); err != nil {
		t.Fatal(err)
	}

	session := &fakePlaybackSession{position: 45}
	model := Model{
		store: saved,
		nowPlaying: session,
		playingVid: &ytdlp.Video{ID: "video-1", Title: "Example", URL: "https://example.test/video"},
	}
	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})
	if updated.(Model).nowPlaying != session {
		t.Fatal("pause key unexpectedly replaced the playback session")
	}

	position, ok := saved.FindHistoryPosition("video-1")
	if !ok || position != 45 {
		t.Fatalf("saved resume position = (%v, %v), want (45, true)", position, ok)
	}
}


func TestExternalPlaybackExitPersistsLastObservedPosition(t *testing.T) {
	t.Setenv("YTUI_DATA_PATH", t.TempDir()+"/data.json")
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := saved.AddHistory(store.HistoryEntry{
		VideoID: "video-exit", Title: "Example", URL: "https://example.test/video", PositionS: 5,
	}); err != nil {
		t.Fatal(err)
	}
	session := &fakePlaybackSession{position: 42}
	model := Model{
		store: saved,
		nowPlaying: session,
		playingVid: &ytdlp.Video{ID: "video-exit", Title: "Example", URL: "https://example.test/video"},
	}
	updated, _ := model.clearNowPlaying()
	if updated.(Model).nowPlaying != nil {
		t.Fatal("playback session was not cleared")
	}
	position, ok := saved.FindHistoryPosition("video-exit")
	if !ok || position != 42 {
		t.Fatalf("saved resume position = (%v, %v), want (42, true)", position, ok)
	}
}
