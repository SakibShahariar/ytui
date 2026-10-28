package ui

import "testing"

type fakePlaybackSession struct {
	waitCalled bool
}

func (f *fakePlaybackSession) Position() (float64, error) { return 0, nil }
func (f *fakePlaybackSession) Duration() (float64, error) { return 0, nil }
func (f *fakePlaybackSession) Volume() (float64, error) { return 0, nil }
func (f *fakePlaybackSession) IsPaused() (bool, error) { return false, nil }
func (f *fakePlaybackSession) Quit() error { return nil }
func (f *fakePlaybackSession) TogglePause() error { return nil }
func (f *fakePlaybackSession) SeekRelative(int) error { return nil }
func (f *fakePlaybackSession) SeekAbsolute(float64) error { return nil }
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
