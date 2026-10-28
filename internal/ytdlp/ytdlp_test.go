package ytdlp

import (
	"encoding/json"
	"os"
	"path/filepath"
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


func TestGetSubtitleTracksListsManualAndAutomaticLanguages(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "yt-dlp")
	output := `{"subtitles":{"fr":[{"name":"French"}],"en":[{"name":"English"}]},"automatic_captions":{"es":[{"name":"Spanish auto"}],"en":[{"name":"Duplicate English"}]}}`
	script := "#!/bin/sh\nprintf '%s\\n' '" + output + "'\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { t.Fatal(err) }
	old := binaryName
	binaryName = bin
	defer func(){ binaryName = old }()
	tracks, err := GetSubtitleTracks("https://www.youtube.com/watch?v=example")
	if err != nil { t.Fatalf("GetSubtitleTracks() error = %v", err) }
	if len(tracks) != 3 { t.Fatalf("tracks length = %d, want 3: %#v", len(tracks), tracks) }
	if tracks[0].Language != "en" || tracks[0].Name != "English" || tracks[0].Automatic { t.Fatalf("first track = %#v", tracks[0]) }
	if tracks[1].Language != "fr" || tracks[1].Name != "French" { t.Fatalf("second track = %#v", tracks[1]) }
	if tracks[2].Language != "es" || !tracks[2].Automatic { t.Fatalf("automatic track = %#v", tracks[2]) }
}

func TestGetSubtitleTracksReportsNoCaptions(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "yt-dlp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' '{\"subtitles\":{},\"automatic_captions\":{}}'\n"), 0o700); err != nil { t.Fatal(err) }
	old := binaryName
	binaryName = bin
	defer func(){ binaryName = old }()
	if _, err := GetSubtitleTracks("https://www.youtube.com/watch?v=example"); err == nil { t.Fatal("expected no-caption error") }
}


func TestParseTranscriptVTTStripsHeadersTimestampsAndMarkup(t *testing.T) {
	input := []byte("WEBVTT\r\n\r\n00:00:00.000 --> 00:00:01.000\r\n<v Speaker>Hello &amp; welcome</v>\r\n\r\n00:00:01.000 --> 00:00:02.000\r\nHello &amp; welcome\r\n\r\n00:00:02.000 --> 00:00:03.000\r\n<c.green>Next line</c>\r\n")
	got := parseTranscriptVTT(input)
	want := "Hello & welcome\nNext line"
	if got != want { t.Fatalf("parseTranscriptVTT() = %q, want %q", got, want) }
}

func TestParseTranscriptVTTEmptyInput(t *testing.T) {
	if got := parseTranscriptVTT([]byte("WEBVTT\n\n")); got != "" { t.Fatalf("empty transcript = %q, want empty", got) }
}
