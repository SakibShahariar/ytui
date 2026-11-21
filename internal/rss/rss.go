// Package rss parses YouTube channel feeds so subscriptions can work without
// the YouTube Data API. It only reads public XML feeds.
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Item is one video from a channel feed.
type Item struct {
	ID        string
	Title     string
	Link      string
	Published time.Time
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	GUID    string `xml:"guid"`
	PubDate string `xml:"pubDate"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	ID        string     `xml:"id"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
}

type atomDocument struct {
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

// VideoID extracts the YouTube video ID from a watch URL or guid.
func VideoID(link string) string {
	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	if parsed.Host == "" {
		return strings.TrimPrefix(link, "yt:video:")
	}
	value := parsed.Query().Get("v")
	if value == "" && strings.Contains(parsed.Host, "youtu.be") {
		value = strings.TrimPrefix(parsed.Path, "/")
	}
	if value == "" {
		value = strings.TrimPrefix(parsed.Path, "/watch/")
	}
	return value
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// Parse reads an RSS 2.0 or Atom feed and returns its video items.
func Parse(data []byte) (title string, items []Item, err error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<?xml") && strings.Contains(trimmed, "<feed") {
		var doc atomDocument
		if err := xml.Unmarshal(data, &doc); err != nil {
			return "", nil, err
		}
		for _, entry := range doc.Entries {
			link := ""
			for _, l := range entry.Links {
				if l.Href != "" {
					link = l.Href
					break
				}
			}
			id := entry.ID
			if strings.HasPrefix(id, "yt:video:") {
				id = strings.TrimPrefix(id, "yt:video:")
			}
			if id == "" {
				id = VideoID(link)
			}
			published := parseTime(entry.Published)
			if published.IsZero() {
				published = parseTime(entry.Updated)
			}
			items = append(items, Item{ID: id, Title: strings.TrimSpace(entry.Title), Link: link, Published: published})
		}
		return strings.TrimSpace(doc.Title), items, nil
	}
	var doc rssDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", nil, err
	}
	for _, entry := range doc.Channel.Items {
		link := strings.TrimSpace(entry.Link)
		id := VideoID(link)
		if id == "" {
			id = strings.TrimSpace(entry.GUID)
		}
		items = append(items, Item{
			ID: id, Title: strings.TrimSpace(entry.Title), Link: link,
			Published: parseTime(entry.PubDate),
		})
	}
	return strings.TrimSpace(doc.Channel.Title), items, nil
}

// Client is the HTTP surface used by Fetch; tests supply a mock.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

// Fetch downloads and parses a feed.
func Fetch(ctx context.Context, client Client, feedURL string) (string, []Item, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "ytui-rss")
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("RSS feed returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	return Parse(data)
}
