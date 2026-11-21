package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/SakibShahariar/ytui/internal/rss"
	"github.com/SakibShahariar/ytui/internal/store"
)

// rssFetch is replaceable in tests.
var rssFetch = rss.Fetch

func runRSSCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return &exitError{code: 2, err: fmt.Errorf("usage: ytui rss add|list|remove|refresh [URL]")}
	}
	s, err := store.Load()
	if err != nil {
		return &exitError{code: 1, err: err}
	}
	switch args[0] {
	case "add":
		name := ""
		rest := args[1:]
		if len(rest) >= 2 && (rest[0] == "--name" || rest[0] == "-n") {
			name, rest = rest[1], rest[2:]
		}
		if len(rest) != 1 {
			return &exitError{code: 2, err: fmt.Errorf("usage: ytui rss add [--name NAME] FEED_URL")}
		}
		feedURL := strings.TrimSpace(rest[0])
		if !strings.HasPrefix(feedURL, "http://") && !strings.HasPrefix(feedURL, "https://") {
			if strings.HasPrefix(feedURL, "UC") && len(feedURL) > 2 {
				feedURL = "https://www.youtube.com/feeds/videos.xml?channel_id=" + url.QueryEscape(feedURL)
			}
		}
		parsed, parseErr := url.Parse(feedURL)
		if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return &exitError{code: 2, err: fmt.Errorf("feed URL must be an absolute https URL")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		title, _, fetchErr := rssFetch(ctx, nil, feedURL)
		if fetchErr != nil {
			return &exitError{code: 1, err: fmt.Errorf("could not read feed: %w", fetchErr)}
		}
		if name == "" {
			name = title
		}
		channelID := parsed.Query().Get("channel_id")
		if err := s.AddRSSFeed(store.RSSFeed{URL: feedURL, Name: name, ChannelID: channelID}); err != nil {
			return &exitError{code: 1, err: err}
		}
		fmt.Fprintf(stdout, "subscribed to %s\n", name)
		return nil
	case "list":
		if len(s.RSSFeeds) == 0 {
			fmt.Fprintln(stdout, "no RSS feeds subscribed")
			return nil
		}
		for _, feed := range s.RSSFeeds {
			state := "ok"
			if feed.LastError != "" {
				state = "error: " + feed.LastError
			}
			fmt.Fprintf(stdout, "%s — %s (last checked: %s; %s)\n", feed.Name, feed.URL, feed.LastChecked.Format(time.RFC3339), state)
		}
		return nil
	case "remove":
		if len(args) != 2 {
			return &exitError{code: 2, err: fmt.Errorf("usage: ytui rss remove FEED_URL")}
		}
		if err := s.RemoveRSSFeed(args[1]); err != nil {
			return &exitError{code: 1, err: err}
		}
		fmt.Fprintf(stdout, "removed %s\n", args[1])
		return nil
	case "refresh":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		total := 0
		for _, feed := range s.RSSFeeds {
			_, items, fetchErr := rssFetch(ctx, nil, feed.URL)
			message := ""
			if fetchErr != nil {
				message = fetchErr.Error()
				fmt.Fprintf(stderr, "%s: %v\n", feed.Name, fetchErr)
			} else {
				total += len(items)
				for _, item := range items {
					fmt.Fprintf(stdout, "%s — %s\n", feed.Name, item.Title)
				}
			}
			if err := s.UpdateRSSFeed(feed.URL, time.Now(), message); err != nil {
				return &exitError{code: 1, err: err}
			}
		}
		fmt.Fprintf(stdout, "RSS refresh complete: %d item(s)\n", total)
		return nil
	default:
		return &exitError{code: 2, err: fmt.Errorf("unknown rss command %q", args[0])}
	}
}
