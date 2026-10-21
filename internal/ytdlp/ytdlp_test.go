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
