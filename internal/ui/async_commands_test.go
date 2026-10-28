package ui

import (
	"errors"
	"testing"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
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
