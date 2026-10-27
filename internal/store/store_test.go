package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveUsesPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not supported on Windows")
	}

	s := &Store{dir: t.TempDir()}
	s.History = []HistoryEntry{{VideoID: "private-video", URL: "https://example.invalid/watch"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(s.path())
	if err != nil {
		t.Fatalf("Stat(data.json) error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("data.json permissions = %#o, want %#o", got, 0o600)
	}
}

func TestAddHistoryCapsAndPrepends(t *testing.T) {
	s := &Store{dir: t.TempDir()}
	for i := 0; i < 500; i++ {
		s.History = append(s.History, HistoryEntry{VideoID: string(rune(i + 1))})
	}

	if err := s.AddHistory(HistoryEntry{VideoID: "newest"}); err != nil {
		t.Fatalf("AddHistory() error = %v", err)
	}
	if got := len(s.History); got != 500 {
		t.Fatalf("history length = %d, want 500", got)
	}
	if got := s.History[0].VideoID; got != "newest" {
		t.Errorf("first history video = %q, want newest", got)
	}
}

func TestAddHistoryUpdatesExistingVideoWithoutDuplicate(t *testing.T) {
	s := &Store{dir: t.TempDir()}
	s.History = []HistoryEntry{{VideoID: "same", Title: "old title"}}
	if err := s.AddHistory(HistoryEntry{VideoID: "same", Title: "new title"}); err != nil {
		t.Fatalf("AddHistory() error = %v", err)
	}
	if len(s.History) != 1 {
		t.Fatalf("history length = %d, want 1", len(s.History))
	}
	if got := s.History[0].Title; got != "new title" {
		t.Errorf("updated title = %q, want new title", got)
	}
}

func TestSubscribeDeduplicatesChannelID(t *testing.T) {
	s := &Store{dir: t.TempDir()}
	channel := Channel{ID: "channel-1", Name: "First name"}
	if err := s.Subscribe(channel); err != nil {
		t.Fatalf("first Subscribe() error = %v", err)
	}
	if err := s.Subscribe(Channel{ID: "channel-1", Name: "Changed name"}); err != nil {
		t.Fatalf("duplicate Subscribe() error = %v", err)
	}
	if len(s.Subscriptions) != 1 {
		t.Fatalf("subscription count = %d, want 1", len(s.Subscriptions))
	}
	if got := s.Subscriptions[0].Name; got != "First name" {
		t.Errorf("duplicate subscription replaced existing entry: name = %q", got)
	}
}

func TestAddWatchLaterDeduplicatesVideoID(t *testing.T) {
	s := &Store{dir: t.TempDir()}
	if err := s.AddWatchLater(WatchLaterEntry{VideoID: "video-1", Title: "Original"}); err != nil {
		t.Fatalf("first AddWatchLater() error = %v", err)
	}
	if err := s.AddWatchLater(WatchLaterEntry{VideoID: "video-1", Title: "Duplicate"}); err != nil {
		t.Fatalf("duplicate AddWatchLater() error = %v", err)
	}
	if len(s.WatchLater) != 1 {
		t.Fatalf("watch-later count = %d, want 1", len(s.WatchLater))
	}
	if got := s.WatchLater[0].Title; got != "Original" {
		t.Errorf("duplicate queue entry replaced existing entry: title = %q", got)
	}
}

func TestPathReturnsDataFilePath(t *testing.T) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configDir, "ytui", "data.json")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}


func TestDataPathOverrideLoadsAndSavesIsolatedState(t *testing.T) {
	override := filepath.Join(t.TempDir(), "portable", "state.json")
	t.Setenv("YTUI_DATA_PATH", override)
	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if path != override {
		t.Fatalf("Path() = %q, want override %q", path, override)
	}
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	s.History = []HistoryEntry{{VideoID: "isolated", Title: "Portable state"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(override); err != nil {
		t.Fatalf("override data file was not written: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload from override error = %v", err)
	}
	if len(loaded.History) != 1 || loaded.History[0].VideoID != "isolated" {
		t.Fatalf("reloaded state = %#v, want isolated history entry", loaded.History)
	}
}
