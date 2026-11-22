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
		DownloadDir:              "/tmp/ytui-downloads",
		DownloadSort:             "progress",
		DownloadStatusFilter:     2,
		MaxConcurrentDownloads:   3,
		DefaultQuality:           "1080p",
		DefaultCookiesBrowser:    "firefox",
		DefaultAudioOnly:         true,
		DisableResumePrompt:      true,
		DesktopNotifications:     true,
		AutoplayNext:             true,
		ThumbnailCacheMaxEntries: 120,
		ThumbnailCacheMaxAgeDays: 14,
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
		{"unsupported download sort", `{"download_sort":"size"}`, "download_sort"},
		{"invalid download status filter", `{"download_status_filter":4}`, "download_status_filter"},
		{"negative concurrent download limit", `{"max_concurrent_downloads":-1}`, "max_concurrent_downloads"},
		{"excessive concurrent download limit", `{"max_concurrent_downloads":11}`, "max_concurrent_downloads"},
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

func TestDownloadRateLimitValidation(t *testing.T) {
	for _, rate := range []string{"", "500K", "1M", "2G", "500k", " 1m "} {
		if err := Validate(Config{DownloadRateLimit: rate}); err != nil {
			t.Errorf("Validate(DownloadRateLimit=%q) error = %v", rate, err)
		}
	}
	for _, rate := range []string{"abc", "0", "-1M", "1T", "1.5M", "M", "1 M"} {
		if err := Validate(Config{DownloadRateLimit: rate}); err == nil {
			t.Errorf("Validate(DownloadRateLimit=%q) unexpectedly succeeded", rate)
		}
	}
	canonical, err := normalizeRateLimit(" 500k ")
	if err != nil || canonical != "500K" {
		t.Fatalf("normalizeRateLimit(500k) = %q, %v; want 500K", canonical, err)
	}
}

func TestThemeValidation(t *testing.T) {
	for _, theme := range []string{"", "terminal", "catppuccin-mocha", "nord", "gruvbox-dark", "dracula", "solarized-dark", "solarized-light", "tokyo-night", "rose-pine", "one-dark", " TERMINAL "} {
		if err := Validate(Config{Theme: theme}); err != nil {
			t.Errorf("Validate(Theme=%q) error = %v", theme, err)
		}
	}
	for _, theme := range []string{"dark", "nord-light"} {
		if err := Validate(Config{Theme: theme}); err == nil {
			t.Errorf("Validate(Theme=%q) unexpectedly succeeded", theme)
		}
	}
	for _, colors := range []map[string]string{
		nil,
		{"mauve": "#f0f", "green": "#00ff00"},
	} {
		if err := Validate(Config{Colors: colors}); err != nil {
			t.Errorf("Validate(Colors=%v) error = %v", colors, err)
		}
	}
	for _, colors := range []map[string]string{
		{"unknown": "#fff"},
		{"mauve": "fff"},
		{"mauve": "#gggggg"},
	} {
		if err := Validate(Config{Colors: colors}); err == nil {
			t.Errorf("Validate(Colors=%v) unexpectedly succeeded", colors)
		}
	}
	cfg := normalize(Config{Theme: " TERMINAL "})
	if cfg.Theme != "terminal" {
		t.Fatalf("normalized theme = %q", cfg.Theme)
	}
}

func TestDownloadPresetsValidation(t *testing.T) {
	valid := []Config{
		{},
		{DownloadPresets: []DownloadPreset{{Name: "Lecture", Quality: "720p", WriteSubs: true}, {Name: "Podcast", Quality: "audio-mp3", RateLimit: "500K"}}},
	}
	for _, cfg := range valid {
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate(%#v) error = %v", cfg, err)
		}
	}
	invalid := []Config{
		{DownloadPresets: []DownloadPreset{{Quality: "720p"}}},
		{DownloadPresets: []DownloadPreset{{Name: "A", Quality: "720p"}, {Name: "a", Quality: "480p"}}},
		{DownloadPresets: []DownloadPreset{{Name: "A", Quality: "2160p"}}},
		{DownloadPresets: []DownloadPreset{{Name: "A", Quality: "720p", RateLimit: "1T"}}},
	}
	for _, cfg := range invalid {
		if err := Validate(cfg); err == nil {
			t.Errorf("Validate(%#v) unexpectedly succeeded", cfg)
		}
	}
	cfg := Config{DownloadPresets: []DownloadPreset{{Name: " Lecture ", Quality: "audio-mp3", RateLimit: "2m"}}}
	normalized := normalize(cfg)
	p := normalized.DownloadPresets[0]
	if p.Name != "Lecture" || p.Quality != "Audio only (mp3)" || p.RateLimit != "2M" {
		t.Fatalf("normalized preset = %#v", p)
	}
}

func TestKeybindingsValidation(t *testing.T) {
	valid := []Config{
		{},
		{Keybindings: map[string]string{"quit": "ctrl+q"}},
		{Keybindings: map[string]string{"download": "F5", "search": "*"}},
	}
	for _, cfg := range valid {
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate(%#v) error = %v", cfg, err)
		}
	}
	invalid := []Config{
		{Keybindings: map[string]string{"unknown": "ctrl+q"}},
		{Keybindings: map[string]string{"quit": "q"}}, // bare letter breaks search typing
		{Keybindings: map[string]string{"quit": ""}},
		{Keybindings: map[string]string{"quit": "ctrl+q", "stop": "ctrl+q"}},
		{Keybindings: map[string]string{"quit": "ctrl+d"}}, // collides with download default
	}
	for _, cfg := range invalid {
		if err := Validate(cfg); err == nil {
			t.Errorf("Validate(%#v) unexpectedly succeeded", cfg)
		}
	}
	cfg := Config{Keybindings: map[string]string{"quit": " CTRL+Q "}}
	normalized := normalize(cfg)
	if got := normalized.Keybindings["quit"]; got != "ctrl+q" {
		t.Fatalf("normalized binding = %q, want ctrl+q", got)
	}
}

func TestDownloadRetryValidation(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{DownloadRetryAttempts: 3, DownloadRetryBackoffSeconds: 5},
		{DownloadRetryAttempts: 10, DownloadRetryBackoffSeconds: 3600},
	} {
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate(%#v) error = %v", cfg, err)
		}
	}
	for _, cfg := range []Config{
		{DownloadRetryAttempts: -1},
		{DownloadRetryAttempts: 11},
		{DownloadRetryBackoffSeconds: -1},
		{DownloadRetryBackoffSeconds: 3601},
	} {
		if err := Validate(cfg); err == nil {
			t.Errorf("Validate(%#v) unexpectedly succeeded", cfg)
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
