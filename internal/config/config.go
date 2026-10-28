// Package config handles ytui's optional user-editable settings file in
// the platform's user configuration directory (XDG_CONFIG_HOME on Linux).
// The file contains preferences only; OAuth credentials and tokens are kept
// separately and are never included in config import/export.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Config holds user-editable defaults. JSON deliberately keeps the format
// dependency-free and easy to inspect or back up.
type Config struct {
	// DownloadDir overrides where downloads are saved. Empty means the
	// downloader package's own default (~/Downloads/ytui).
	DownloadDir string `json:"download_dir,omitempty"`

	// DefaultQuality pre-selects a row in the ctrl+d download-options
	// picker by stable identifier or legacy display label. Empty means
	// "Best quality (video+audio)".
	DefaultQuality string `json:"default_quality,omitempty"`

	// DefaultCookiesBrowser overrides browser auto-detection (e.g. "firefox").
	// Empty means auto-detect.
	DefaultCookiesBrowser string `json:"default_cookies_browser,omitempty"`

	// DefaultAudioOnly sets the initial audio-only playback toggle.
	DefaultAudioOnly bool `json:"default_audio_only,omitempty"`

	// DesktopNotifications enables best-effort desktop notifications for download completion/failure.
	DesktopNotifications bool `json:"desktop_notifications,omitempty"`

	// AutoplayNext starts the next Watch Later item when playback ends.
	AutoplayNext bool `json:"autoplay_next,omitempty"`

	// SponsorBlockEnabled enables privacy-preserving segment lookup during playback.
	SponsorBlockEnabled bool `json:"sponsorblock_enabled,omitempty"`
	// SponsorBlockCategories lists categories to skip automatically. When not configured,
	// ytui defaults to sponsor and selfpromo; an explicit empty list disables auto-skip.
	SponsorBlockCategories []string `json:"sponsorblock_categories,omitempty"`
	SponsorBlockCategoriesConfigured bool `json:"sponsorblock_categories_configured,omitempty"`

	// ThumbnailCacheMaxEntries bounds the number of cached thumbnails; zero uses the default.
	ThumbnailCacheMaxEntries int `json:"thumbnail_cache_max_entries,omitempty"`

	// ThumbnailCacheMaxAgeDays expires cached thumbnails after this many days; zero uses the default.
	ThumbnailCacheMaxAgeDays int `json:"thumbnail_cache_max_age_days,omitempty"`
}

var qualityLabels = map[string]string{
	"best": "Best quality (video+audio)",
	"best-quality": "Best quality (video+audio)",
	"best_quality": "Best quality (video+audio)",
	"best quality (video+audio)": "Best quality (video+audio)",
	"1080p": "1080p",
	"720p": "720p",
	"480p": "480p",
	"audio-mp3": "Audio only (mp3)",
	"audio_mp3": "Audio only (mp3)",
	"audio only (mp3)": "Audio only (mp3)",
	"audio-opus": "Audio only (opus)",
	"audio_opus": "Audio only (opus)",
	"audio only (opus)": "Audio only (opus)",
}

// normalizeQuality converts stable machine-readable identifiers to the
// corresponding picker label. Existing display-label values remain supported
// for backwards compatibility.
func normalizeQuality(value string) string {
	if label, ok := qualityLabels[strings.ToLower(strings.TrimSpace(value))]; ok {
		return label
	}
	return value
}

// Validate checks user-provided fields before an imported config can replace
// the active settings. Empty values mean "use the built-in default".
func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.DownloadDir) != "" {
		for _, r := range cfg.DownloadDir {
			if unicode.IsControl(r) {
				return fmt.Errorf("download_dir must not contain control characters")
			}
		}
	}
	if strings.TrimSpace(cfg.DefaultQuality) != "" {
		if _, ok := qualityLabels[strings.ToLower(strings.TrimSpace(cfg.DefaultQuality))]; !ok {
			return fmt.Errorf("default_quality %q is not supported", cfg.DefaultQuality)
		}
	}
	if strings.TrimSpace(cfg.DefaultCookiesBrowser) != "" {
		browser := strings.ToLower(strings.TrimSpace(cfg.DefaultCookiesBrowser))
		switch browser {
		case "firefox", "chrome", "chromium", "brave", "edge":
		default:
			return fmt.Errorf("default_cookies_browser %q is not supported (use firefox, chrome, chromium, brave, or edge)", cfg.DefaultCookiesBrowser)
		}
	}
	if cfg.ThumbnailCacheMaxEntries < 0 || cfg.ThumbnailCacheMaxEntries > 10000 {
		return fmt.Errorf("thumbnail_cache_max_entries must be between 1 and 10000, or 0 for the default")
	}
	if cfg.ThumbnailCacheMaxAgeDays < 0 || cfg.ThumbnailCacheMaxAgeDays > 3650 {
		return fmt.Errorf("thumbnail_cache_max_age_days must be between 1 and 3650, or 0 for the default")
	}
	return nil
}

func normalize(cfg Config) Config {
	cfg.DefaultQuality = normalizeQuality(cfg.DefaultQuality)
	cfg.DefaultCookiesBrowser = strings.ToLower(strings.TrimSpace(cfg.DefaultCookiesBrowser))
	return cfg
}

func path() (string, error) {
	if override := os.Getenv("YTUI_CONFIG_PATH"); override != "" {
		p, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve YTUI_CONFIG_PATH: %w", err)
		}
		return filepath.Clean(p), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ytui", "config.json"), nil
}

// Path returns the active config file path.
func Path() string {
	p, _ := path()
	return p
}

// Load reads the config file, returning defaults if it is missing or invalid.
// Use LoadWithError when the caller needs to explain why defaults were used.
func Load() Config {
	cfg, _ := LoadWithError()
	return cfg
}

// LoadWithError returns defaults for a missing config file. If an existing
// config cannot be read, parsed, or validated, it returns defaults alongside
// an error so the application can surface the problem without failing startup.
func LoadWithError() (Config, error) {
	p, err := path()
	if err != nil {
		return Config{}, fmt.Errorf("resolve config path: %w", err)
	}
	cfg, err := read(p)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", p, err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", p, err)
	}
	return cfg, nil
}

func read(p string) (Config, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg = normalize(cfg)
	return cfg, nil
}

func write(p string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config.json-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

// Save writes the config atomically to the active config path.
func Save(cfg Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	return write(p, cfg)
}

// Export writes the current effective settings to a JSON file. It exports
// preferences only, never OAuth credentials, tokens, subscriptions, or history.
func Export(destination string) error {
	if destination == "" {
		return os.ErrInvalid
	}
	return write(destination, Load())
}

// Import validates a JSON settings file before replacing the active settings.
// Invalid input leaves the existing config untouched.
func Import(source string) error {
	if source == "" {
		return os.ErrInvalid
	}
	cfg, err := read(source)
	if err != nil {
		return err
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	return Save(cfg)
}
