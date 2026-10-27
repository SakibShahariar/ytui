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

func TestNormalizeQuality(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"best", "Best quality (video+audio)"},
		{"best-quality", "Best quality (video+audio)"},
		{"1080p", "1080p"},
		{"720p", "720p"},
		{"480p", "480p"},
		{"audio-mp3", "Audio only (mp3)"},
		{"audio-opus", "Audio only (opus)"},
		{" Audio_MP3 ", "Audio only (mp3)"},
		{"Audio only (opus)", "Audio only (opus)"},
		{"unknown-value", "unknown-value"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeQuality(tt.input); got != tt.want {
				t.Errorf("normalizeQuality(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReadNormalizesStableQualityIdentifier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"default_quality":"audio-mp3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultQuality != "Audio only (mp3)" {
		t.Fatalf("DefaultQuality = %q, want %q", got.DefaultQuality, "Audio only (mp3)")
	}
}
