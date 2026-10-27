package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestHomeFeedCacheRoundTripAndAge(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	videos := []ytdlp.Video{{ID: "one", Title: "Cached video", URL: "https://example.com/video"}}
	if err := saveHomeFeedCache(videos, "account-a"); err != nil {
		t.Fatalf("saveHomeFeedCache() error = %v", err)
	}
	got, age, err := loadHomeFeedCache("account-a")
	if err != nil {
		t.Fatalf("loadHomeFeedCache() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != videos[0].ID || got[0].Title != videos[0].Title {
		t.Fatalf("cached videos = %#v, want %#v", got, videos)
	}
	if age < 0 || age > time.Minute {
		t.Fatalf("cache age = %v, want a recent timestamp", age)
	}
	path, err := homeFeedCachePath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache permissions = %o, want private file permissions", info.Mode().Perm())
	}
}

func TestHomeFeedCacheAgeSupportsStaleRefreshDecision(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path, err := homeFeedCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	cached := homeFeedCache{
		CachedAt: time.Now().Add(-homeFeedCacheTTL - time.Minute),
		SubscriptionKey: "account-a",
		Videos: []ytdlp.Video{{ID: "stale", Title: "Stale video"}},
	}
	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, age, err := loadHomeFeedCache()
	if err != nil {
		t.Fatalf("loadHomeFeedCache() error = %v", err)
	}
	if age < homeFeedCacheTTL {
		t.Fatalf("cache age = %v, want age >= TTL %v", age, homeFeedCacheTTL)
	}
}

func TestHomeFeedCacheRejectsCorruptAndEmptyFiles(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path, err := homeFeedCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadHomeFeedCache(); err == nil {
		t.Fatal("corrupt cache should be rejected")
	}
	if err := saveHomeFeedCache(nil, "account-a"); err == nil {
		t.Fatal("empty feed should not be cached")
	}
}

func TestHomeFeedErrorMessageExplainsQuotaAndAuthorizationFailures(t *testing.T) {
	for _, input := range []string{"quota exceeded", "HTTP 403 Forbidden"} {
		got := homeFeedErrorMessage(fakeError(input))
		if !strings.Contains(got, "quota or authorization") {
			t.Errorf("homeFeedErrorMessage(%q) = %q", input, got)
		}
	}
	if got := homeFeedErrorMessage(fakeError("temporary network failure")); got != "temporary network failure" {
		t.Errorf("ordinary error was rewritten: %q", got)
	}
}

type fakeError string
func (e fakeError) Error() string { return string(e) }
