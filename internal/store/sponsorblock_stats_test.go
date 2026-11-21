package store

import (
	"path/filepath"
	"testing"
)

func newStatsStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSponsorBlockStatsAccumulateAndPersist(t *testing.T) {
	s := newStatsStore(t)
	if err := s.RecordSponsorBlockSkip("sponsor", 10.5); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSponsorBlockSkip("sponsor", 4.5); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSponsorBlockSkip("selfpromo", 2); err != nil {
		t.Fatal(err)
	}
	skips, seconds, byCategory := s.SponsorBlockSummary()
	if skips != 3 || seconds != 17 || len(byCategory) != 2 {
		t.Fatalf("summary = %d skips, %v seconds, %v", skips, seconds, byCategory)
	}
	if byCategory[0].Category != "selfpromo" || byCategory[1].Category != "sponsor" {
		t.Fatalf("categories not sorted: %v", byCategory)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	skips, seconds, _ = reloaded.SponsorBlockSummary()
	if skips != 3 || seconds != 17 {
		t.Fatalf("persisted summary = %d skips, %v seconds", skips, seconds)
	}
}

func TestResetSponsorBlockStats(t *testing.T) {
	s := newStatsStore(t)
	if err := s.RecordSponsorBlockSkip("sponsor", 5); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetSponsorBlockStats(); err != nil {
		t.Fatal(err)
	}
	skips, seconds, byCategory := s.SponsorBlockSummary()
	if skips != 0 || seconds != 0 || len(byCategory) != 0 {
		t.Fatalf("after reset = %d skips, %v seconds, %v", skips, seconds, byCategory)
	}
}
