package thumb

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCacheDir(t *testing.T) string {
	t.Helper()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	dir := filepath.Join(cacheHome, "ytui", "thumbs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheMu.Lock()
	activePreviewPath = ""
	cacheMu.Unlock()
	t.Cleanup(func() {
		cacheMu.Lock()
		activePreviewPath = ""
		cacheMu.Unlock()
	})
	return dir
}

func writeCacheFile(t *testing.T, dir, name string, size int, age time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanupCacheExpiresOldEntriesAndBoundsEntryCount(t *testing.T) {
	dir := testCacheDir(t)
	writeCacheFile(t, dir, "expired.jpg", 5, 40*24*time.Hour)
	writeCacheFile(t, dir, "old.jpg", 7, 10*24*time.Hour)
	writeCacheFile(t, dir, "newer.jpg", 11, time.Hour)
	writeCacheFile(t, dir, "newest.jpg", 13, time.Minute)

	stats, err := CleanupCache(CachePolicy{MaxEntries: 2, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("CleanupCache() error = %v", err)
	}
	if stats.Entries != 2 || stats.Bytes != 24 {
		t.Fatalf("CacheStats = %#v, want 2 entries / 24 bytes", stats)
	}
	for _, name := range []string{"expired.jpg", "old.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s remains after cleanup, stat err = %v", name, err)
		}
	}
}

func TestCleanupCachePreservesActivePreview(t *testing.T) {
	dir := testCacheDir(t)
	active := writeCacheFile(t, dir, "active.jpg", 3, 90*24*time.Hour)
	writeCacheFile(t, dir, "other.jpg", 4, time.Hour)
	cacheMu.Lock()
	activePreviewPath = active
	cacheMu.Unlock()

	stats, err := CleanupCache(CachePolicy{MaxEntries: 1, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("CleanupCache() error = %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active preview was removed: %v", err)
	}
	if stats.Entries != 1 || stats.Bytes != 3 {
		t.Fatalf("CacheStats = %#v, want active 1 entry / 3 bytes", stats)
	}
}

func TestCleanupCacheRemovesStaleTemporaryFiles(t *testing.T) {
	dir := testCacheDir(t)
	stale := writeCacheFile(t, dir, ".thumb-stale.tmp", 2, 48*time.Hour)
	fresh := writeCacheFile(t, dir, ".thumb-fresh.tmp", 2, time.Hour)
	if _, err := CleanupCache(CachePolicy{}); err != nil {
		t.Fatalf("CleanupCache() error = %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale temp file remains, stat err = %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh temp file was removed: %v", err)
	}
}


func TestCleanupCacheProtectsRequestedPreviewDuringTransition(t *testing.T) {
	dir := testCacheDir(t)
	active := writeCacheFile(t, dir, "active.jpg", 3, 90*24*time.Hour)
	requested := writeCacheFile(t, dir, "requested.jpg", 4, 60*24*time.Hour)
	cacheMu.Lock()
	activePreviewPath = active
	cacheMu.Unlock()

	if _, err := cleanupCache(CachePolicy{MaxEntries: 1, MaxAgeDays: 30}, requested); err != nil {
		t.Fatalf("cleanupCache() error = %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("current preview was removed before replacement: %v", err)
	}
	if _, err := os.Stat(requested); err != nil {
		t.Fatalf("requested preview was removed before display: %v", err)
	}

	cacheMu.Lock()
	activePreviewPath = requested
	cacheMu.Unlock()
	stats, err := CleanupCache(CachePolicy{MaxEntries: 1, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("CleanupCache() after transition error = %v", err)
	}
	if _, err := os.Stat(requested); err != nil {
		t.Fatalf("new active preview was removed: %v", err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("replaced preview remains after cleanup, stat err = %v", err)
	}
	if stats.Entries != 1 || stats.Bytes != 4 {
		t.Fatalf("CacheStats = %#v, want requested 1 entry / 4 bytes", stats)
	}
}


func TestGetCacheStatsReportsCurrentUsage(t *testing.T) {
	dir := testCacheDir(t)
	writeCacheFile(t, dir, "one.jpg", 6, time.Hour)
	writeCacheFile(t, dir, "two.jpg", 9, time.Minute)
	writeCacheFile(t, dir, "ignored.tmp", 30, time.Minute)

	stats, err := GetCacheStats()
	if err != nil {
		t.Fatalf("GetCacheStats() error = %v", err)
	}
	if stats.Directory != dir || stats.Entries != 2 || stats.Bytes != 15 {
		t.Fatalf("GetCacheStats() = %#v, want directory %q, 2 entries, 15 bytes", stats, dir)
	}
}
