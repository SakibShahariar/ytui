// Package config handles ytui's optional user-editable settings file in
// the platform's user configuration directory (XDG_CONFIG_HOME on Linux).
// The file contains preferences only; OAuth credentials and tokens are kept
// separately and are never included in config import/export.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-editable defaults. JSON deliberately keeps the format
// dependency-free and easy to inspect or back up.
type Config struct {
	// DownloadDir overrides where downloads are saved. Empty means the
	// downloader package's own default (~/Downloads/ytui).
	DownloadDir string `json:"download_dir,omitempty"`

	// DefaultQuality pre-selects a row in the ctrl+d download-options
	// picker by label. Empty means "Best quality (video+audio)".
	DefaultQuality string `json:"default_quality,omitempty"`

	// DefaultCookiesBrowser overrides browser auto-detection (e.g. "firefox").
	// Empty means auto-detect.
	DefaultCookiesBrowser string `json:"default_cookies_browser,omitempty"`

	// DefaultAudioOnly sets the initial audio-only playback toggle.
	DefaultAudioOnly bool `json:"default_audio_only,omitempty"`
}

func path() (string, error) {
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
func Load() Config {
	p, err := path()
	if err != nil {
		return Config{}
	}
	cfg, err := read(p)
	if err != nil {
		return Config{}
	}
	return cfg
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
	return Save(cfg)
}
