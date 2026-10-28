package ytdlp

import (
	"encoding/json"
	"testing"
)

func TestVideoJSONUsesUsefulThumbnailResolution(t *testing.T) {
	const raw = `{
		"id": "abc123",
		"title": "Example",
		"thumbnails": [
			{"url": "https://example.invalid/small.jpg", "width": 120, "height": 90},
			{"url": "https://example.invalid/medium.jpg", "width": 320, "height": 180},
			{"url": "https://example.invalid/large.jpg", "width": 1280, "height": 720}
		]
	}`

	var data videoJSON
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	got := data.toVideo().Thumbnail
	want := "https://example.invalid/medium.jpg"
	if got != want {
		t.Errorf("Thumbnail = %q, want %q", got, want)
	}
}

func TestVideoJSONFallsBackToYouTubeThumbnail(t *testing.T) {
	got := (videoJSON{ID: "abc123"}).toVideo().Thumbnail
	want := "https://i.ytimg.com/vi/abc123/mqdefault.jpg"
	if got != want {
		t.Errorf("Thumbnail = %q, want %q", got, want)
	}
}

func TestVideoJSONDirectURLFallbacks(t *testing.T) {
	video := (videoJSON{}).toVideo()
	if video.ID != "" || video.Title != "" {
		t.Fatalf("empty metadata unexpectedly filled ID/title: %#v", video)
	}
}


func TestIsPlaylistURL(t *testing.T) {
	valid := []string{
		"https://www.youtube.com/playlist?list=PLabc123",
		"https://youtube.com/watch?v=video123&list=PLabc123",
		"https://youtu.be/video123?list=PLabc123",
		"https://music.youtube.com/playlist?list=PLabc123",
	}
	for _, raw := range valid {
		if !IsPlaylistURL(raw) { t.Errorf("IsPlaylistURL(%q) = false, want true", raw) }
	}
	invalid := []string{
		"https://www.youtube.com/watch?v=video123",
		"https://example.com/playlist?list=PLabc123",
		"not a URL",
		"file:///tmp/list?list=abc",
	}
	for _, raw := range invalid {
		if IsPlaylistURL(raw) { t.Errorf("IsPlaylistURL(%q) = true, want false", raw) }
	}
}

func TestParsePlaylistJSONPreservesOrderAndSkipsMissingIDs(t *testing.T) {
	const raw = `{"title":"Road Trip Mix","entries":[
		{"id":"first","title":"First track","channel":"Alpha"},
		{"title":"Unavailable entry"},
		{"id":"second","title":"Second track","channel":"Beta","webpage_url":"https://www.youtube.com/watch?v=second"}
	]}`
	title, videos, err := parsePlaylistJSON([]byte(raw))
	if err != nil { t.Fatalf("parsePlaylistJSON() error = %v", err) }
	if title != "Road Trip Mix" { t.Fatalf("title = %q, want Road Trip Mix", title) }
	if len(videos) != 2 { t.Fatalf("videos length = %d, want 2", len(videos)) }
	if videos[0].ID != "first" || videos[0].PlaylistIndex != 1 { t.Fatalf("first entry = %#v", videos[0]) }
	if videos[0].URL != "https://www.youtube.com/watch?v=first" { t.Fatalf("first URL = %q", videos[0].URL) }
	if videos[1].ID != "second" || videos[1].PlaylistIndex != 2 { t.Fatalf("second entry = %#v", videos[1]) }
}

func TestParsePlaylistJSONRejectsEmptyAccessiblePlaylist(t *testing.T) {
	_, _, err := parsePlaylistJSON([]byte(`{"title":"Private mix","entries":[{"title":"Unavailable"}]}`))
	if err == nil { t.Fatal("empty accessible playlist unexpectedly succeeded") }
}
