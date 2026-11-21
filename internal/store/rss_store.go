package store

import (
	"fmt"
	"strings"
	"time"
)

// RSSFeed is a channel feed followed without the YouTube Data API.
type RSSFeed struct {
	URL         string    `json:"url"`
	ChannelID   string    `json:"channel_id,omitempty"`
	Name        string    `json:"name"`
	AddedAt     time.Time `json:"added_at"`
	LastChecked time.Time `json:"last_checked,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// AddRSSFeed stores a new feed, ignoring duplicates by URL.
func (s *Store) AddRSSFeed(feed RSSFeed) error {
	feed.URL = strings.TrimSpace(feed.URL)
	if feed.URL == "" {
		return fmt.Errorf("feed URL is empty")
	}
	for _, existing := range s.RSSFeeds {
		if existing.URL == feed.URL {
			return fmt.Errorf("feed %q is already subscribed", feed.URL)
		}
	}
	if feed.AddedAt.IsZero() {
		feed.AddedAt = time.Now()
	}
	s.RSSFeeds = append(s.RSSFeeds, feed)
	return s.Save()
}

// RemoveRSSFeed removes a feed by URL.
func (s *Store) RemoveRSSFeed(feedURL string) error {
	kept := make([]RSSFeed, 0, len(s.RSSFeeds))
	for _, feed := range s.RSSFeeds {
		if feed.URL != feedURL {
			kept = append(kept, feed)
		}
	}
	if len(kept) == len(s.RSSFeeds) {
		return fmt.Errorf("feed %q is not subscribed", feedURL)
	}
	s.RSSFeeds = kept
	return s.Save()
}

// UpdateRSSFeed records the outcome of a refresh.
func (s *Store) UpdateRSSFeed(feedURL string, checked time.Time, feedError string) error {
	for i := range s.RSSFeeds {
		if s.RSSFeeds[i].URL == feedURL {
			s.RSSFeeds[i].LastChecked = checked
			s.RSSFeeds[i].LastError = feedError
			return s.Save()
		}
	}
	return fmt.Errorf("feed %q is not subscribed", feedURL)
}
