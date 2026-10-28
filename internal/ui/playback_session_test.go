package ui

import (
	"errors"
	"testing"
)

type fakePlaybackSession struct {
	waitCalled bool
	seekTo     float64
	seekCalled bool
}

func (f *fakePlaybackSession) Position() (float64, error) { return 0, nil }
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
