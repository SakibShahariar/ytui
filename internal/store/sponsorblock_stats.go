package store

import (
	"math"
	"sort"
	"strings"
)

// SponsorBlockStat records locally how often a segment category was skipped
// and roughly how much playback time that saved. No video identifiers or
// network data are stored here.
type SponsorBlockStat struct {
	Category     string  `json:"category"`
	Skips        int     `json:"skips"`
	SecondsSaved float64 `json:"seconds_saved"`
}

// RecordSponsorBlockSkip accumulates a skip for one category and persists it.
func (s *Store) RecordSponsorBlockSkip(category string, seconds float64) error {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil
	}
	if seconds < 0 {
		seconds = 0
	}
	for i := range s.SponsorBlockStats {
		if s.SponsorBlockStats[i].Category == category {
			s.SponsorBlockStats[i].Skips++
			s.SponsorBlockStats[i].SecondsSaved = math.Round((s.SponsorBlockStats[i].SecondsSaved+seconds)*10) / 10
			return s.Save()
		}
	}
	s.SponsorBlockStats = append(s.SponsorBlockStats, SponsorBlockStat{
		Category: category, Skips: 1, SecondsSaved: math.Round(seconds*10) / 10,
	})
	return s.Save()
}

// ResetSponsorBlockStats clears all locally stored skip statistics.
func (s *Store) ResetSponsorBlockStats() error {
	s.SponsorBlockStats = nil
	return s.Save()
}

// SponsorBlockSummary returns totals and a category-sorted copy of the stats.
func (s *Store) SponsorBlockSummary() (skips int, seconds float64, byCategory []SponsorBlockStat) {
	byCategory = append([]SponsorBlockStat(nil), s.SponsorBlockStats...)
	sort.Slice(byCategory, func(i, j int) bool { return byCategory[i].Category < byCategory[j].Category })
	for _, stat := range byCategory {
		skips += stat.Skips
		seconds += stat.SecondsSaved
	}
	return
}
