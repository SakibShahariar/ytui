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
	"strconv"
	"strings"
	"unicode"
)

// Config holds user-editable defaults. JSON deliberately keeps the format
// dependency-free and easy to inspect or back up.
type Config struct {
	// DownloadDir overrides where downloads are saved. Empty means the
	// downloader package's own default (~/Downloads/ytui).
	DownloadDir string `json:"download_dir,omitempty"`

	// DownloadSort selects the live download queue ordering.
	DownloadSort string `json:"download_sort,omitempty"`

	// DownloadStatusFilter selects all, active, finished, or failed downloads.
	DownloadStatusFilter int `json:"download_status_filter,omitempty"`

	// MaxConcurrentDownloads bounds simultaneous yt-dlp processes; zero defaults to one.
	MaxConcurrentDownloads int `json:"max_concurrent_downloads,omitempty"`

	// DownloadRateLimit is the global yt-dlp rate limit (for example "500K"
	// or "2M"). Empty means no limit. Individual downloads can override it.
	DownloadRateLimit string `json:"download_rate_limit,omitempty"`

	// DownloadRetryAttempts is how many times a transient download failure is
	// retried automatically. Zero disables automatic retries.
	DownloadRetryAttempts int `json:"download_retry_attempts,omitempty"`

	// DownloadRetryBackoffSeconds is the base delay before the first retry,
	// doubled on each subsequent attempt. Zero uses the built-in default.
	DownloadRetryBackoffSeconds int `json:"download_retry_backoff_seconds,omitempty"`

	// DefaultQuality pre-selects a row in the ctrl+d download-options
	// picker by stable identifier or legacy display label. Empty means
	// "Best quality (video+audio)".
	DefaultQuality string `json:"default_quality,omitempty"`

	// DefaultCookiesBrowser overrides browser auto-detection (e.g. "firefox").
	// Empty means auto-detect.
	DefaultCookiesBrowser string `json:"default_cookies_browser,omitempty"`

	// DefaultAudioOnly sets the initial audio-only playback toggle.
	DefaultAudioOnly bool `json:"default_audio_only,omitempty"`

	// DisableResumePrompt starts videos from the beginning instead of prompting.
	DisableResumePrompt bool `json:"disable_resume_prompt,omitempty"`

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

	// Keybindings maps action names to key strings, overriding the defaults in
	// DefaultKeybindings. Only global shortcuts are remappable; typing and
	// list-navigation keys stay fixed.
	Keybindings map[string]string `json:"keybindings,omitempty"`

	// DownloadPresets are user-defined download option bundles shown alongside
	// the built-in presets in the Ctrl+D picker.
	DownloadPresets []DownloadPreset `json:"download_presets,omitempty"`

	// MouseEnabled opts into mouse wheel scrolling and click-to-select in the
	// TUI. Keyboard controls are unaffected either way.
	MouseEnabled bool `json:"mouse_enabled,omitempty"`

	// Theme selects a built-in color profile: default, minimal,
	// high-contrast, or light. Empty uses the default.
	Theme string `json:"theme,omitempty"`

	// Colors overrides individual palette roles (for example "mauve" or
	// "green") with #RGB or #RRGGBB values, on top of the selected theme.
	Colors map[string]string `json:"colors,omitempty"`

	// UpdateCheckEnabled opts into checking GitHub releases for a newer
	// version. ytui never downloads or installs updates automatically.
	UpdateCheckEnabled bool `json:"update_check_enabled,omitempty"`

	// UpdateCheckIntervalHours is the minimum interval between automatic
	// update checks. Zero uses the built-in default.
	UpdateCheckIntervalHours int `json:"update_check_interval_hours,omitempty"`

	// RSSRefreshIntervalHours is the minimum hours between automatic RSS
	// feed refreshes. Zero disables automatic refresh; use the palette or
	// CLI to refresh on demand.
	RSSRefreshIntervalHours int `json:"rss_refresh_interval_hours,omitempty"`
}

// DownloadPreset bundles quality and post-processing choices for quick reuse.
type DownloadPreset struct {
	Name           string `json:"name"`
	Quality        string `json:"quality"`
	WriteSubs      bool   `json:"write_subs,omitempty"`
	EmbedThumbnail bool   `json:"embed_thumbnail,omitempty"`
	RateLimit      string `json:"rate_limit,omitempty"`
}

// DefaultKeybindings lists the global shortcuts that can be remapped. Keys
// used for typing or list navigation (Enter, arrows, Esc) are intentionally
// absent: the TUI relies on them in many contexts.
var DefaultKeybindings = map[string]string{
	"quit":            "ctrl+c",
	"search":          "/",
	"command_palette": ":",
	"help":            "?",
	"next_tab":        "tab",
	"prev_tab":        "shift+tab",
	"audio":           "ctrl+a",
	"watch_later":     "ctrl+w",
	"download":        "ctrl+d",
	"subtitles":       "ctrl+v",
	"transcript":      "ctrl+e",
	"sponsorblock":    "ctrl+b",
	"thumbnails":      "ctrl+t",
	"pause":           "ctrl+p",
	"stop":            "ctrl+x",
	"login":           "ctrl+l",
	"subscribe":       "ctrl+s",
	"retry":           "ctrl+r",
}

// BuiltinThemes are the selectable color profiles.
var BuiltinThemes = []string{"default", "minimal", "high-contrast", "light", "terminal"}

// ColorRoles are the palette roles that can be overridden in config.json.
var ColorRoles = []string{
	"base", "mantle", "text", "subtext1", "subtext0", "overlay1", "overlay0",
	"surface2", "surface1", "mauve", "pink", "red", "peach", "yellow",
	"green", "blue", "lavender",
}

var namedKeyBindings = map[string]bool{
	"enter": true, "tab": true, "shift+tab": true, "esc": true,
	"space": true, "pgup": true, "pgdown": true, "home": true,
	"end": true, "delete": true, "backspace": true,
	"f1": true, "f2": true, "f3": true, "f4": true, "f5": true,
	"f6": true, "f7": true, "f8": true, "f9": true, "f10": true,
	"f11": true, "f12": true,
}

// validKeyBinding rejects bare letters and digits: those are used for typing
// search queries and list shortcuts, so binding one globally would make the
// interface unusable while typing.
func validKeyBinding(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return false
	}
	if namedKeyBindings[v] {
		return true
	}
	if strings.HasPrefix(v, "ctrl+") || strings.HasPrefix(v, "alt+") {
		key := strings.TrimPrefix(strings.TrimPrefix(v, "ctrl+"), "alt+")
		runes := []rune(key)
		return len(runes) == 1 && (unicode.IsLetter(runes[0]) || unicode.IsDigit(runes[0]))
	}
	runes := []rune(v)
	return len(runes) == 1 && unicode.IsPrint(runes[0]) && !unicode.IsLetter(runes[0]) && !unicode.IsDigit(runes[0])
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
// NormalizeRateLimit validates and canonicalizes a yt-dlp --limit-rate value
// for callers outside the config package (for example the headless CLI).
func NormalizeRateLimit(value string) (string, error) {
	return normalizeRateLimit(value)
}

// normalizeRateLimit validates a yt-dlp --limit-rate value and returns its
// canonical form. Accepted values are a positive integer with an optional
// K, M, or G suffix (case-insensitive), matching yt-dlp's size syntax.
func normalizeRateLimit(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	digits := value
	suffix := ""
	if last := value[len(value)-1]; last == 'K' || last == 'k' || last == 'M' || last == 'm' || last == 'G' || last == 'g' {
		digits = value[:len(value)-1]
		suffix = strings.ToUpper(string(last))
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 {
		return "", fmt.Errorf("download_rate_limit %q is not a valid rate (use a positive number with optional K, M, or G suffix)", value)
	}
	return strconv.Itoa(n) + suffix, nil
}

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
	switch cfg.DownloadSort {
	case "", "added", "title", "channel", "progress", "status":
	default:
		return fmt.Errorf("download_sort %q is not supported", cfg.DownloadSort)
	}
	if cfg.MaxConcurrentDownloads < 0 || cfg.MaxConcurrentDownloads > 10 {
		return fmt.Errorf("max_concurrent_downloads must be between 1 and 10, or 0 for the default")
	}
	if _, err := normalizeRateLimit(cfg.DownloadRateLimit); err != nil {
		return err
	}
	if cfg.DownloadRetryAttempts < 0 || cfg.DownloadRetryAttempts > 10 {
		return fmt.Errorf("download_retry_attempts must be between 0 and 10")
	}
	if cfg.DownloadRetryBackoffSeconds < 0 || cfg.DownloadRetryBackoffSeconds > 3600 {
		return fmt.Errorf("download_retry_backoff_seconds must be between 0 and 3600")
	}
	if cfg.UpdateCheckIntervalHours < 0 || cfg.UpdateCheckIntervalHours > 8760 {
		return fmt.Errorf("update_check_interval_hours must be between 0 and 8760")
	}
	if cfg.RSSRefreshIntervalHours < 0 || cfg.RSSRefreshIntervalHours > 8760 {
		return fmt.Errorf("rss_refresh_interval_hours must be between 0 and 8760")
	}
	if cfg.DownloadStatusFilter < 0 || cfg.DownloadStatusFilter > 3 {
		return fmt.Errorf("download_status_filter must be between 0 and 3")
	}
	if cfg.ThumbnailCacheMaxEntries < 0 || cfg.ThumbnailCacheMaxEntries > 10000 {
		return fmt.Errorf("thumbnail_cache_max_entries must be between 1 and 10000, or 0 for the default")
	}
	if cfg.ThumbnailCacheMaxAgeDays < 0 || cfg.ThumbnailCacheMaxAgeDays > 3650 {
		return fmt.Errorf("thumbnail_cache_max_age_days must be between 1 and 3650, or 0 for the default")
	}
	effective := make(map[string]string, len(DefaultKeybindings))
	for action, key := range DefaultKeybindings {
		effective[action] = key
	}
	for action, binding := range cfg.Keybindings {
		if _, ok := DefaultKeybindings[action]; !ok {
			return fmt.Errorf("keybindings: unknown action %q", action)
		}
		if !validKeyBinding(binding) {
			return fmt.Errorf("keybindings: invalid key %q for action %q", binding, action)
		}
		effective[action] = strings.ToLower(strings.TrimSpace(binding))
	}
	seen := make(map[string]string, len(effective))
	for action, binding := range effective {
		if other, exists := seen[binding]; exists {
			return fmt.Errorf("keybindings: actions %q and %q both use %q", other, action, binding)
		}
		seen[binding] = action
	}
	if theme := strings.ToLower(strings.TrimSpace(cfg.Theme)); theme != "" {
		found := false
		for _, builtin := range BuiltinThemes {
			if theme == builtin {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("theme %q is not supported (use default, minimal, high-contrast, light, or terminal)", cfg.Theme)
		}
	}
	validRoles := make(map[string]bool, len(ColorRoles))
	for _, role := range ColorRoles {
		validRoles[role] = true
	}
	for role, color := range cfg.Colors {
		if !validRoles[strings.ToLower(strings.TrimSpace(role))] {
			return fmt.Errorf("colors: unknown palette role %q", role)
		}
		value := strings.TrimSpace(color)
		if len(value) != 4 && len(value) != 7 {
			return fmt.Errorf("colors: %q must be a #RGB or #RRGGBB hex value", color)
		}
		if value[0] != '#' {
			return fmt.Errorf("colors: %q must start with #", color)
		}
		for _, ch := range value[1:] {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return fmt.Errorf("colors: %q is not a valid hex color", color)
			}
		}
	}
	presetNames := make(map[string]bool, len(cfg.DownloadPresets))
	for _, preset := range cfg.DownloadPresets {
		name := strings.TrimSpace(preset.Name)
		if name == "" {
			return fmt.Errorf("download_presets: preset name must not be empty")
		}
		lower := strings.ToLower(name)
		if presetNames[lower] {
			return fmt.Errorf("download_presets: duplicate preset name %q", name)
		}
		presetNames[lower] = true
		if _, ok := qualityLabels[strings.ToLower(strings.TrimSpace(preset.Quality))]; !ok {
			return fmt.Errorf("download_presets: preset %q has unsupported quality %q", name, preset.Quality)
		}
		if _, err := normalizeRateLimit(preset.RateLimit); err != nil {
			return fmt.Errorf("download_presets: preset %q: %w", name, err)
		}
	}
	return nil
}

func normalize(cfg Config) Config {
	cfg.Theme = strings.ToLower(strings.TrimSpace(cfg.Theme))
	cfg.DefaultQuality = normalizeQuality(cfg.DefaultQuality)
	cfg.DefaultCookiesBrowser = strings.ToLower(strings.TrimSpace(cfg.DefaultCookiesBrowser))
	if rate, err := normalizeRateLimit(cfg.DownloadRateLimit); err == nil {
		cfg.DownloadRateLimit = rate
	}
	if len(cfg.Keybindings) > 0 {
		bindings := make(map[string]string, len(cfg.Keybindings))
		for action, binding := range cfg.Keybindings {
			bindings[action] = strings.ToLower(strings.TrimSpace(binding))
		}
		cfg.Keybindings = bindings
	}
	if len(cfg.DownloadPresets) > 0 {
		presets := make([]DownloadPreset, len(cfg.DownloadPresets))
		copy(presets, cfg.DownloadPresets)
		for i := range presets {
			presets[i].Name = strings.TrimSpace(presets[i].Name)
			presets[i].Quality = normalizeQuality(presets[i].Quality)
			if rate, err := normalizeRateLimit(presets[i].RateLimit); err == nil {
				presets[i].RateLimit = rate
			}
		}
		cfg.DownloadPresets = presets
	}
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
