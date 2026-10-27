package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestImportExportRoundTrip(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	want := Config{
		DownloadDir:           "/tmp/ytui-downloads",
		DefaultQuality:        "1080p",
		DefaultCookiesBrowser: "firefox",
		DefaultAudioOnly:          true,
		ThumbnailCacheMaxEntries:  120,
		ThumbnailCacheMaxAgeDays:  14,
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

func TestImportInvalidFieldDoesNotReplaceConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := Config{DefaultQuality: "720p", DefaultCookiesBrowser: "firefox"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		json string
		want string
	}{
		{"quality", `{"default_quality":"8k"}`, "default_quality"},
		{"browser", `{"default_cookies_browser":"opera"}`, "default_cookies_browser"},
		{"download path control character", "{\"download_dir\":\"bad\\u0000path\"}", "download_dir"},
		{"negative thumbnail entry limit", `{"thumbnail_cache_max_entries":-1}`, "thumbnail_cache_max_entries"},
		{"excessive thumbnail age", `{"thumbnail_cache_max_age_days":3651}`, "thumbnail_cache_max_age_days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(input, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			err := Import(input)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Import() error = %v, want field-specific error containing %q", err, tc.want)
			}
			if got := Load(); !reflect.DeepEqual(got, want) {
				t.Fatalf("invalid import changed config: got %#v, want %#v", got, want)
			}
		})
	}
}

func TestValidateAcceptsOptionalAndLegacyValues(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{DefaultQuality: "Best quality (video+audio)", DefaultCookiesBrowser: "Firefox"},
		{DefaultQuality: "audio-mp3", DefaultCookiesBrowser: " edge "},
	} {
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate(%#v) error = %v", cfg, err)
		}
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


func TestLoadWithErrorMissingFileUsesDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := LoadWithError()
	if err != nil {
		t.Fatalf("LoadWithError() error = %v, want nil for missing config", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("LoadWithError() = %#v, want zero-value defaults", got)
	}
}

func TestLoadWithErrorReportsCorruptConfigAndUsesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	path := filepath.Join(home, "ytui", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadWithError()
	if err == nil || !strings.Contains(err.Error(), "config.json") {
		t.Fatalf("LoadWithError() error = %v, want config path context", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("LoadWithError() = %#v, want safe defaults", got)
	}
}

func TestLoadWithErrorReportsInvalidConfigValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	path := filepath.Join(home, "ytui", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"default_quality":"8k"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWithError(); err == nil || !strings.Contains(err.Error(), "default_quality") {
		t.Fatalf("LoadWithError() error = %v, want invalid quality reported", err)
	}
}


func TestConfigPathOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "portable", "settings.json")
	t.Setenv("YTUI_CONFIG_PATH", override)
	got, err := path()
	if err != nil {
		t.Fatalf("path() error = %v", err)
	}
	if got != override {
		t.Fatalf("path() = %q, want override %q", got, override)
	}
}
