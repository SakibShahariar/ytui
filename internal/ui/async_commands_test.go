package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
	"github.com/SakibShahariar/ytui/internal/sponsorblock"
)

func TestDoSearchWithUsesInjectedSearch(t *testing.T) {
	want := []ytdlp.Video{{ID: "abc123", Title: "Test video"}}
	called := false

	msg := doSearchWith("ambient music", func(query string, limit int) ([]ytdlp.Video, error) {
		called = true
		if query != "ambient music" {
			t.Fatalf("query = %q, want %q", query, "ambient music")
		}
		if limit != 25 {
			t.Fatalf("limit = %d, want 25", limit)
		}
		return want, nil
	})()

	if !called {
		t.Fatal("injected search function was not called")
	}
	got, ok := msg.(searchResultsMsg)
	if !ok {
		t.Fatalf("message type = %T, want searchResultsMsg", msg)
	}
	if got.err != nil {
		t.Fatalf("unexpected search error: %v", got.err)
	}
	if len(got.results) != 1 || got.results[0] != want[0] {
		t.Fatalf("results = %#v, want %#v", got.results, want)
	}
}

func TestDoSearchWithPreservesSearchError(t *testing.T) {
	wantErr := errors.New("search unavailable")

	msg := doSearchWith("test", func(string, int) ([]ytdlp.Video, error) {
		return nil, wantErr
	})()

	got, ok := msg.(searchResultsMsg)
	if !ok {
		t.Fatalf("message type = %T, want searchResultsMsg", msg)
	}
	if !errors.Is(got.err, wantErr) {
		t.Fatalf("error = %v, want %v", got.err, wantErr)
	}
	if len(got.results) != 0 {
		t.Fatalf("results = %#v, want no results", got.results)
	}
}

func TestLoadDirectURLWithUsesInjectedLookup(t *testing.T) {
	want := ytdlp.Video{ID: "xyz789", Title: "Direct video"}
	called := false

	msg := loadDirectURLWith("https://example.com/watch", func(videoURL string) (ytdlp.Video, error) {
		called = true
		if videoURL != "https://example.com/watch" {
			t.Fatalf("URL = %q, want original URL", videoURL)
		}
		return want, nil
	})()

	got, ok := msg.(directURLMsg)
	if !ok {
		t.Fatalf("message type = %T, want directURLMsg", msg)
	}
	if !called || got.video != want || got.err != nil {
		t.Fatalf("lookup result = %#v, called = %v; want video %#v and no error", got, called, want)
	}
}

func TestFetchDescriptionWithUsesInjectedLookup(t *testing.T) {
	called := false
	msg := fetchDescriptionWith("video-id", "https://example.com/watch", func(videoURL string) (string, error) {
		called = true
		if videoURL != "https://example.com/watch" {
			t.Fatalf("URL = %q, want original URL", videoURL)
		}
		return "sample description", nil
	})()

	got, ok := msg.(videoDescriptionMsg)
	if !ok {
		t.Fatalf("message type = %T, want videoDescriptionMsg", msg)
	}
	if !called || got.videoID != "video-id" || got.desc != "sample description" || got.err != nil {
		t.Fatalf("description result = %#v, called = %v", got, called)
	}
}

func TestLoadDirectURLWithPreservesLookupError(t *testing.T) {
	wantErr := errors.New("metadata unavailable")
	msg := loadDirectURLWith("https://example.com/watch", func(string) (ytdlp.Video, error) {
		return ytdlp.Video{}, wantErr
	})()

	got, ok := msg.(directURLMsg)
	if !ok {
		t.Fatalf("message type = %T, want directURLMsg", msg)
	}
	if !errors.Is(got.err, wantErr) {
		t.Fatalf("error = %v, want %v", got.err, wantErr)
	}
}

func TestFetchDescriptionWithPreservesLookupError(t *testing.T) {
	wantErr := errors.New("description unavailable")
	msg := fetchDescriptionWith("video-id", "https://example.com/watch", func(string) (string, error) {
		return "", wantErr
	})()

	got, ok := msg.(videoDescriptionMsg)
	if !ok {
		t.Fatalf("message type = %T, want videoDescriptionMsg", msg)
	}
	if got.videoID != "video-id" {
		t.Fatalf("video ID = %q, want video-id", got.videoID)
	}
	if !errors.Is(got.err, wantErr) {
		t.Fatalf("error = %v, want %v", got.err, wantErr)
	}
}


func TestLoadPlaylistWithUsesInjectedLoader(t *testing.T) {
	want := []ytdlp.Video{{ID: "first", Title: "First", PlaylistIndex: 1}}
	called := false
	msg := loadPlaylistWith("https://www.youtube.com/playlist?list=PLexample", func(rawURL string) (string, []ytdlp.Video, error) {
		called = true
		if rawURL != "https://www.youtube.com/playlist?list=PLexample" { t.Fatalf("URL = %q, want original playlist URL", rawURL) }
		return "Road Trip Mix", want, nil
	})()
	got, ok := msg.(searchResultsMsg)
	if !ok { t.Fatalf("message type = %T, want searchResultsMsg", msg) }
	if !called || got.err != nil || got.playlistTitle != "Road Trip Mix" || len(got.results) != 1 || got.results[0] != want[0] {
		t.Fatalf("playlist result = %#v, called = %v", got, called)
	}
}

func TestLoadPlaylistWithPreservesError(t *testing.T) {
	wantErr := errors.New("playlist unavailable")
	msg := loadPlaylistWith("https://www.youtube.com/playlist?list=PLexample", func(string) (string, []ytdlp.Video, error) {
		return "", nil, wantErr
	})()
	got, ok := msg.(searchResultsMsg)
	if !ok { t.Fatalf("message type = %T, want searchResultsMsg", msg) }
	if !errors.Is(got.err, wantErr) { t.Fatalf("error = %v, want %v", got.err, wantErr) }
}

func TestLoadTranscriptWithUsesSelectedLanguage(t *testing.T) {
	video := ytdlp.Video{ID: "video-id", Title: "Transcript video", URL: "https://youtube.com/watch?v=video-id"}
	called := false
	msg := loadTranscriptWith(video, "fr", func(rawURL, language string) (string, error) {
		called = true
		if rawURL != video.URL { t.Fatalf("URL = %q, want %q", rawURL, video.URL) }
		if language != "fr" { t.Fatalf("language = %q, want fr", language) }
		return "Bonjour", nil
	})()
	got, ok := msg.(transcriptMsg)
	if !ok { t.Fatalf("message type = %T, want transcriptMsg", msg) }
	if !called || got.err != nil || got.text != "Bonjour" || got.video.ID != video.ID {
		t.Fatalf("transcript result = %#v, called = %v", got, called)
	}
}

func TestLoadTranscriptWithPreservesError(t *testing.T) {
	wantErr := errors.New("captions unavailable")
	msg := loadTranscriptWith(ytdlp.Video{URL: "https://youtube.com/watch?v=video-id"}, "en", func(string, string) (string, error) {
		return "", wantErr
	})()
	got, ok := msg.(transcriptMsg)
	if !ok { t.Fatalf("message type = %T, want transcriptMsg", msg) }
	if !errors.Is(got.err, wantErr) { t.Fatalf("error = %v, want %v", got.err, wantErr) }
}

func TestLoadSponsorSegmentsWithUsesInjectedFetcher(t *testing.T) {
	want := []sponsorblock.Segment{{Start: 10, End: 20, Category: "sponsor", ActionType: "skip"}}
	called := false
	msg := loadSponsorSegmentsWith("video-id", []string{"sponsor"}, func(ctx context.Context, videoID string, categories []string) ([]sponsorblock.Segment, error) {
		called = true
		if videoID != "video-id" { t.Fatalf("video ID = %q", videoID) }
		if len(categories) != 1 || categories[0] != "sponsor" { t.Fatalf("categories = %#v", categories) }
		return want, nil
	})()
	got, ok := msg.(sponsorSegmentsMsg)
	if !ok { t.Fatalf("message type = %T, want sponsorSegmentsMsg", msg) }
	if !called || got.err != nil || len(got.segments) != 1 || got.segments[0] != want[0] {
		t.Fatalf("SponsorBlock result = %#v, called = %v", got, called)
	}
}

func TestLoadSponsorSegmentsWithPreservesErrors(t *testing.T) {
	wantErr := errors.New("API unavailable")
	msg := loadSponsorSegmentsWith("video-id", []string{"sponsor"}, func(context.Context, string, []string) ([]sponsorblock.Segment, error) {
		return nil, wantErr
	})()
	got, ok := msg.(sponsorSegmentsMsg)
	if !ok { t.Fatalf("message type = %T, want sponsorSegmentsMsg", msg) }
	if !errors.Is(got.err, wantErr) { t.Fatalf("error = %v, want %v", got.err, wantErr) }
}
