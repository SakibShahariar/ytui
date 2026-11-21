package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckParsesRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v0.1.0","html_url":"https://example.com/release","published_at":"2026-11-01T00:00:00Z","body":"notes"}`))
	}))
	defer server.Close()
	release, err := Check(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v0.1.0" || !strings.Contains(release.HTMLURL, "example.com") || release.PublishedAt.Year() != 2026 {
		t.Fatalf("release = %#v", release)
	}
}

func TestCheckReportsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	if _, err := Check(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestCheckNoReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	if _, err := Check(context.Background(), server.Client(), server.URL); err != ErrNoReleases {
		t.Fatalf("error = %v, want ErrNoReleases", err)
	}
}

func TestNewerThan(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.0.0-20261104-abc", "v0.1.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.2.0", "v0.1.0", false},
		{"dev", "v0.1.0", true},
	} {
		if got := NewerThan(tt.current, tt.latest); got != tt.want {
			t.Errorf("NewerThan(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestShouldCheckRespectsInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-check.json")
	now := time.Now()
	if !ShouldCheck(path, time.Hour, now) {
		t.Fatal("first check should always run")
	}
	if err := RecordCheck(path, now); err != nil {
		t.Fatal(err)
	}
	if ShouldCheck(path, time.Hour, now.Add(30*time.Minute)) {
		t.Fatal("check ran before the interval elapsed")
	}
	if !ShouldCheck(path, time.Hour, now.Add(2*time.Hour)) {
		t.Fatal("check did not run after the interval elapsed")
	}
}
