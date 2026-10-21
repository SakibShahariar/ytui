package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImportExportRoundTrip(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	want := Config{
		DownloadDir:           "/tmp/ytui-downloads",
		DefaultQuality:        "1080p",
		DefaultCookiesBrowser: "firefox",
		DefaultAudioOnly:      true,
	}
	input := filepath.Join(t.TempDir(), "settings.json")
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Import(input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got := Load(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() after import = %#v, want %#v", got, want)
	}

	exported := filepath.Join(t.TempDir(), "backup", "settings.json")
	if err := Export(exported); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	got, err := read(exported)
	if err != nil {
		t.Fatalf("read exported settings: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exported config = %#v, want %#v", got, want)
	}
	info, err := os.Stat(exported)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Errorf("exported file mode = %04o, want 0600", gotMode)
	}
}

func TestImportInvalidFileDoesNotReplaceConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := Config{DefaultQuality: "720p"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}

	input := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(input, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Import(input); err == nil {
		t.Fatal("Import() succeeded for invalid JSON")
	}
	if got := Load(); !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid import changed config: got %#v, want %#v", got, want)
	}
}
