package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRSS(t *testing.T) {
	feed := `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Example Channel</title>
<item><title>First Video</title><link>https://www.youtube.com/watch?v=abc123</link><pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate></item>
<item><title>Second Video</title><link>https://www.youtube.com/watch?v=def456</link><guid>yt:video:def456</guid></item>
</channel></rss>`
	title, items, err := Parse([]byte(feed))
	if err != nil {
		t.Fatal(err)
	}
	if title != "Example Channel" || len(items) != 2 {
		t.Fatalf("title = %q, items = %d", title, len(items))
	}
	if items[0].ID != "abc123" || items[1].ID != "def456" || items[0].Published.IsZero() {
		t.Fatalf("items = %#v", items)
	}
}

func TestParseAtom(t *testing.T) {
	feed := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<title>Atom Channel</title>
<entry><title>Atom Video</title><link href="https://www.youtube.com/watch?v=atom01"/><id>yt:video:atom01</id><published>2026-01-02T03:04:05Z</published></entry>
</feed>`
	title, items, err := Parse([]byte(feed))
	if err != nil {
		t.Fatal(err)
	}
	if title != "Atom Channel" || len(items) != 1 || items[0].ID != "atom01" || items[0].Link == "" {
		t.Fatalf("title = %q, items = %#v", title, items)
	}
}

func TestFetchUsesClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "ytui-rss" {
			t.Errorf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		w.Write([]byte(`<rss><channel><title>T</title><item><title>V</title><link>https://www.youtube.com/watch?v=v123</link></item></channel></rss>`))
	}))
	defer server.Close()
	_, items, err := Fetch(context.Background(), server.Client(), server.URL)
	if err != nil || len(items) != 1 || items[0].ID != "v123" {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
}

func TestVideoIDHandlesNonWatchLinks(t *testing.T) {
	if got := VideoID("https://youtu.be/xyz789"); got != "xyz789" {
		t.Fatalf("short link id = %q", got)
	}
	if got := VideoID("yt:video:xyz789"); got != "xyz789" {
		t.Fatalf("guid id = %q", got)
	}
	if !strings.Contains(VideoID("https://www.youtube.com/watch?v=abc"), "abc") {
		t.Fatal("missing video id")
	}
}
