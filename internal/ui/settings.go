package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/config"
)

// settingsField describes one config value exposed in the in-app settings
// view. Advanced fields stay in config.json only.
type settingsField struct {
	name string
	kind string // string, int, or bool
}

var settingsFields = []settingsField{
	{name: "theme", kind: "string"},
	{name: "download_dir", kind: "string"},
	{name: "default_quality", kind: "string"},
	{name: "download_rate_limit", kind: "string"},
	{name: "default_cookies_browser", kind: "string"},
	{name: "max_concurrent_downloads", kind: "int"},
	{name: "download_retry_attempts", kind: "int"},
	{name: "download_retry_backoff_seconds", kind: "int"},
	{name: "thumbnail_cache_max_entries", kind: "int"},
	{name: "thumbnail_cache_max_age_days", kind: "int"},
	{name: "default_audio_only", kind: "bool"},
	{name: "desktop_notifications", kind: "bool"},
	{name: "autoplay_next", kind: "bool"},
	{name: "sponsorblock_enabled", kind: "bool"},
	{name: "mouse_enabled", kind: "bool"},
}

func settingsValue(cfg config.Config, name string) string {
	switch name {
	case "theme":
		if cfg.Theme == "" {
			return "default"
		}
		return cfg.Theme
	case "download_dir":
		if cfg.DownloadDir == "" {
			return "(default: ~/Downloads/ytui)"
		}
		return cfg.DownloadDir
	case "default_quality":
		if cfg.DefaultQuality == "" {
			return "Best quality (video+audio)"
		}
		return cfg.DefaultQuality
	case "download_rate_limit":
		if cfg.DownloadRateLimit == "" {
			return "(unlimited)"
		}
		return cfg.DownloadRateLimit
	case "default_cookies_browser":
		if cfg.DefaultCookiesBrowser == "" {
			return "(auto-detect)"
		}
		return cfg.DefaultCookiesBrowser
	case "max_concurrent_downloads":
		return strconv.Itoa(cfg.MaxConcurrentDownloads)
	case "download_retry_attempts":
		return strconv.Itoa(cfg.DownloadRetryAttempts)
	case "download_retry_backoff_seconds":
		return strconv.Itoa(cfg.DownloadRetryBackoffSeconds)
	case "thumbnail_cache_max_entries":
		return strconv.Itoa(cfg.ThumbnailCacheMaxEntries)
	case "thumbnail_cache_max_age_days":
		return strconv.Itoa(cfg.ThumbnailCacheMaxAgeDays)
	case "default_audio_only":
		return strconv.FormatBool(cfg.DefaultAudioOnly)
	case "desktop_notifications":
		return strconv.FormatBool(cfg.DesktopNotifications)
	case "autoplay_next":
		return strconv.FormatBool(cfg.AutoplayNext)
	case "sponsorblock_enabled":
		return strconv.FormatBool(cfg.SponsorBlockEnabled)
	case "mouse_enabled":
		return strconv.FormatBool(cfg.MouseEnabled)
	}
	return ""
}

// settingsSet validates the candidate before saving, so a bad value leaves
// the active config untouched.
func settingsSet(cfg *config.Config, name, value string) error {
	candidate := *cfg
	value = strings.TrimSpace(value)
	switch name {
	case "theme":
		candidate.Theme = value
	case "download_dir":
		candidate.DownloadDir = value
	case "default_quality":
		candidate.DefaultQuality = value
	case "download_rate_limit":
		candidate.DownloadRateLimit = value
	case "default_cookies_browser":
		candidate.DefaultCookiesBrowser = value
	case "max_concurrent_downloads", "download_retry_attempts", "download_retry_backoff_seconds",
		"thumbnail_cache_max_entries", "thumbnail_cache_max_age_days":
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be a whole number", name)
		}
		switch name {
		case "max_concurrent_downloads":
			candidate.MaxConcurrentDownloads = number
		case "download_retry_attempts":
			candidate.DownloadRetryAttempts = number
		case "download_retry_backoff_seconds":
			candidate.DownloadRetryBackoffSeconds = number
		case "thumbnail_cache_max_entries":
			candidate.ThumbnailCacheMaxEntries = number
		case "thumbnail_cache_max_age_days":
			candidate.ThumbnailCacheMaxAgeDays = number
		}
	case "default_audio_only", "desktop_notifications", "autoplay_next", "sponsorblock_enabled", "mouse_enabled":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s must be true or false", name)
		}
		switch name {
		case "default_audio_only":
			candidate.DefaultAudioOnly = enabled
		case "desktop_notifications":
			candidate.DesktopNotifications = enabled
		case "autoplay_next":
			candidate.AutoplayNext = enabled
		case "sponsorblock_enabled":
			candidate.SponsorBlockEnabled = enabled
		case "mouse_enabled":
			candidate.MouseEnabled = enabled
		}
	default:
		return fmt.Errorf("unknown setting %q", name)
	}
	if err := config.Validate(candidate); err != nil {
		return err
	}
	if err := config.Save(candidate); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	*cfg = candidate
	return nil
}

func (m Model) settingsContent(width int) string {
	if width < 20 {
		width = 20
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("Settings")
	path := lipgloss.NewStyle().Foreground(ctpOverlay0).Render("Config: " + config.Path())
	lines := []string{heading, path, "", fmt.Sprintf("Advanced options remain in %s", config.Path()), ""}
	for i, field := range settingsFields {
		marker := "  "
		if i == m.settingsCursor {
			marker = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).Render("▸ ")
		}
		value := settingsValue(m.cfg, field.name)
		if i == m.settingsCursor && m.settingsEditing {
			value = m.settingsInput.View()
		}
		lines = append(lines, marker+fmt.Sprintf("%-30s %s", field.name, value))
	}
	lines = append(lines, "", "↑↓: select · enter: edit/toggle · esc: close")
	return strings.Join(lines, "\n")
}

// openSettings prepares the settings overlay with a fresh input.
func (m Model) openSettings() (tea.Model, tea.Cmd) {
	m.settingsView = true
	m.settingsCursor = 0
	m.settingsEditing = false
	m.settingsInput = textinput.New()
	return m, nil
}

// handleSettingsKey routes keys while the settings overlay is open.
func (m Model) handleSettingsKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	field := settingsFields[m.settingsCursor]
	if m.settingsEditing {
		switch key {
		case "enter":
			if err := settingsSet(&m.cfg, field.name, m.settingsInput.Value()); err != nil {
				m.errMsg = "setting not saved: " + err.Error()
			} else {
				m.errMsg = ""
				m.statusMsg = field.name + " updated"
				m.settingsEditing = false
				m.settingsInput.Blur()
			}
			return m, nil
		case "esc":
			m.settingsEditing = false
			m.settingsInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			return m, cmd
		}
	}
	switch key {
	case "esc", "q":
		m.settingsView = false
		m.settingsEditing = false
		return m, nil
	case "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down":
		if m.settingsCursor < len(settingsFields)-1 {
			m.settingsCursor++
		}
		return m, nil
	case "enter", " ":
		if field.kind == "bool" {
			next := strconv.FormatBool(!strings.EqualFold(settingsValue(m.cfg, field.name), "true"))
			if err := settingsSet(&m.cfg, field.name, next); err != nil {
				m.errMsg = "setting not saved: " + err.Error()
			} else {
				m.errMsg = ""
				m.statusMsg = field.name + " = " + next
			}
			return m, nil
		}
		m.settingsEditing = true
		m.settingsInput.SetValue(settingsValue(m.cfg, field.name))
		m.settingsInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}
