// Package config handles ytui's optional user-editable settings file at
// ~/.config/ytui/config.json. There's no in-app editor for it (yet) — the
// person edits the file directly with a text editor, same as
// client_secret.json. Every field has a sane default, so a missing or
// partial file is never an error.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-editable defaults. JSON (not TOML/YAML) deliberately —
// it needs zero extra dependencies since encoding/json is already used
// elsewhere in this codebase (see internal/store), and this file is small
// and simple enough that JSON's minor readability cost doesn't matter.
type Config struct {
	// DownloadDir overrides where downloads are saved. Empty means the
	// downloader package's own default (~/Downloads/ytui).
	DownloadDir string `json:"download_dir,omitempty"`

	// DefaultQuality pre-selects a row in the ctrl+d download-options
	// picker by label (must match one of the labels shown there, e.g.
	// "1080p", "Audio only (mp3)"). Empty means "Best quality (video+audio)".
	DefaultQuality string `json:"default_quality,omitempty"`

	// DefaultCookiesBrowser overrides browser auto-detection in the
	// download picker (e.g. "firefox", "chrome"). Empty means auto-detect.
	DefaultCookiesBrowser string `json:"default_cookies_browser,omitempty"`

	// DefaultAudioOnly sets the initial audio-only playback toggle on
	// startup (can still be changed in-session with ctrl+a).
	DefaultAudioOnly bool `json:"default_audio_only,omitempty"`
}

func path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ytui", "config.json"), nil
}

// Path is exposed so the UI can tell the person where to edit it.
func Path() string {
	p, _ := path()
	return p
}

// Load reads the config file, returning defaults (zero value) if it
// doesn't exist or fails to parse — a broken/missing config should never
// crash the app, just fall back silently.
func Load() Config {
	p, err := path()
	if err != nil {
		return Config{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// Save writes the config file, creating ~/.config/ytui if needed. Not
// currently called by the app itself (no in-app editor yet) — exposed for
// completeness and potential future use.
func Save(cfg Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
