package store

import (
	"os"
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
