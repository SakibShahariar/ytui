package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRSSFeedCrud(t *testing.T) {
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	feed := RSSFeed{URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UC123", Name: "Channel"}
	if err := s.AddRSSFeed(feed); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRSSFeed(feed); err == nil {
		t.Fatal("duplicate feed unexpectedly accepted")
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.RSSFeeds) != 1 || reloaded.RSSFeeds[0].Name != "Channel" {
		t.Fatalf("persisted feeds = %#v", reloaded.RSSFeeds)
	}
	if err := reloaded.UpdateRSSFeed(feed.URL, time.Now(), "HTTP 500"); err != nil {
		t.Fatal(err)
	}
	if reloaded.RSSFeeds[0].LastError != "HTTP 500" {
		t.Fatalf("last error = %q", reloaded.RSSFeeds[0].LastError)
	}
	if err := reloaded.RemoveRSSFeed(feed.URL); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.RSSFeeds) != 0 {
		t.Fatalf("feeds after remove = %#v", reloaded.RSSFeeds)
	}
}
