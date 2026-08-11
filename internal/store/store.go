// Package store handles local persistence: subscriptions, watch history,
// and a watch-later queue. Plain JSON files under ~/.config/ytui for now —
// easy to inspect/edit by hand, and simple enough that we don't need
// SQLite (and its cgo baggage) until the data model actually needs queries.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Channel is a subscribed YouTube channel.
type Channel struct {
	ID   string `json:"id"`   // channel ID or @handle
	Name string `json:"name"`
	URL  string `json:"url"`
}

// HistoryEntry records a watched video.
type HistoryEntry struct {
	VideoID   string    `json:"video_id"`
	Title     string    `json:"title"`
	Channel   string    `json:"channel"`
	URL       string    `json:"url"`
	Thumbnail string    `json:"thumbnail"`
	WatchedAt time.Time `json:"watched_at"`
	PositionS float64   `json:"position_s"` // last known playback position, for resume
}

// WatchLaterEntry is a queued video.
type WatchLaterEntry struct {
	VideoID   string    `json:"video_id"`
	Title     string    `json:"title"`
	Channel   string    `json:"channel"`
	URL       string    `json:"url"`
	Thumbnail string    `json:"thumbnail"`
	AddedAt   time.Time `json:"added_at"`
}

// Store bundles all local state and handles disk I/O.
type Store struct {
	dir string

	Subscriptions []Channel         `json:"subscriptions"`
	History       []HistoryEntry    `json:"history"`
	WatchLater    []WatchLaterEntry `json:"watch_later"`
}

func configDir() (string, error) {
	base, err := os.UserConfigDir() // ~/.config on Linux
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ytui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *Store) path() string {
	return filepath.Join(s.dir, "data.json")
}

// Load reads existing state from disk, or returns a fresh empty Store if
// none exists yet (first run).
func Load() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir}

	data, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return s, nil // fresh install, nothing to load yet
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Save writes the current state to disk atomically (write to temp file,
// then rename) so a crash mid-write can't corrupt the data file.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

// Subscribe adds a channel if not already present.
func (s *Store) Subscribe(c Channel) error {
	for _, existing := range s.Subscriptions {
		if existing.ID == c.ID {
			return nil // already subscribed
		}
	}
	s.Subscriptions = append(s.Subscriptions, c)
	return s.Save()
}

// Unsubscribe removes a channel by ID.
func (s *Store) Unsubscribe(channelID string) error {
	out := s.Subscriptions[:0]
	for _, c := range s.Subscriptions {
		if c.ID != channelID {
			out = append(out, c)
		}
	}
	s.Subscriptions = out
	return s.Save()
}

// AddHistory records (or updates, if already present) a watched video.
// FindHistoryPosition returns the last saved playback position for a
// video, if any — used to offer resuming where you left off.
func (s *Store) FindHistoryPosition(videoID string) (float64, bool) {
	for _, e := range s.History {
		if e.VideoID == videoID {
			return e.PositionS, e.PositionS > 0
		}
	}
	return 0, false
}

func (s *Store) AddHistory(e HistoryEntry) error {
	e.WatchedAt = time.Now()
	for i, existing := range s.History {
		if existing.VideoID == e.VideoID {
			s.History[i] = e
			return s.Save()
		}
	}
	// Prepend so most-recent shows first; cap history length.
	s.History = append([]HistoryEntry{e}, s.History...)
	if len(s.History) > 500 {
		s.History = s.History[:500]
	}
	return s.Save()
}

// AddWatchLater queues a video.
func (s *Store) AddWatchLater(e WatchLaterEntry) error {
	for _, existing := range s.WatchLater {
		if existing.VideoID == e.VideoID {
			return nil
		}
	}
	e.AddedAt = time.Now()
	s.WatchLater = append(s.WatchLater, e)
	return s.Save()
}

// RemoveWatchLater removes a video from the queue.
func (s *Store) RemoveWatchLater(videoID string) error {
	out := s.WatchLater[:0]
	for _, e := range s.WatchLater {
		if e.VideoID != videoID {
			out = append(out, e)
		}
	}
	s.WatchLater = out
	return s.Save()
}
