package library

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestScanFiltersAndSortsMediaFiles(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "older.MP4")
	newer := filepath.Join(dir, "newer.opus")
	ignored := filepath.Join(dir, "notes.txt")
	partial := filepath.Join(dir, ".partial.mp4")
	subdir := filepath.Join(dir, "folder")

	for _, path := range []string{older, newer, ignored, partial} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	got, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	names := make([]string, len(got))
	for i, file := range got {
		names[i] = file.Name
	}
	if want := []string{"newer.opus", "older.MP4"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Scan() names = %v, want %v", names, want)
	}
	if got[0].Size != 1 || got[0].Path != newer {
		t.Fatalf("unexpected file metadata: %#v", got[0])
	}
}

func TestScanMissingDirectoryReturnsError(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Scan() error = nil for missing directory")
	}
}

func TestDeleteRemovesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(File{Path: path}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists or stat failed unexpectedly: %v", err)
	}
}
