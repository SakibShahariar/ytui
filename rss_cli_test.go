package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/rss"
	"github.com/SakibShahariar/ytui/internal/store"
)

func rssTestEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("YTUI_DATA_PATH", filepath.Join(dir, "data.json"))
	original := rssFetch
	rssFetch = func(ctx context.Context, client rss.Client, feedURL string) (string, []rss.Item, error) {
		return "Test Channel", []rss.Item{{ID: "v1", Title: "Video One", Link: "https://www.youtube.com/watch?v=v1"}}, nil
	}
	t.Cleanup(func() { rssFetch = original })
	return dir
}

func TestRSSAddListRefreshRemove(t *testing.T) {
	rssTestEnv(t)
	feedURL := "https://www.youtube.com/feeds/videos.xml?channel_id=UC123"
	var out, errOut strings.Builder
	if err := runRSSCommand([]string{"add", "--name", "Test", feedURL}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.RSSFeeds) != 1 || s.RSSFeeds[0].ChannelID != "UC123" {
		t.Fatalf("feeds = %#v", s.RSSFeeds)
	}
	out.Reset()
	if err := runRSSCommand([]string{"list"}, &out, &errOut); err != nil || !strings.Contains(out.String(), "Test") {
		t.Fatalf("list = %q, err = %v", out.String(), err)
	}
	out.Reset()
	if err := runRSSCommand([]string{"refresh"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Video One") || !strings.Contains(out.String(), "1 item(s)") {
		t.Fatalf("refresh = %q", out.String())
	}
	if err := runRSSCommand([]string{"remove", feedURL}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	s, _ = store.Load()
	if len(s.RSSFeeds) != 0 {
		t.Fatalf("feeds after remove = %#v", s.RSSFeeds)
	}
}

func TestRSSAddAcceptsChannelID(t *testing.T) {
	rssTestEnv(t)
	var out, errOut strings.Builder
	if err := runRSSCommand([]string{"add", "UCabc123"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Load()
	if len(s.RSSFeeds) != 1 || !strings.Contains(s.RSSFeeds[0].URL, "channel_id=UCabc123") {
		t.Fatalf("feeds = %#v", s.RSSFeeds)
	}
}

func TestRSSAddRejectsBadURL(t *testing.T) {
	rssTestEnv(t)
	var out, errOut strings.Builder
	err := runRSSCommand([]string{"add", "http://example.com/feed"}, &out, &errOut)
	var exitErr *exitError
	if !errors.As(err, &exitErr) || exitErr.code != 2 {
		t.Fatalf("error = %v, want exit code 2", err)
	}
}
