package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/config"
)

func settingsTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	return Model{cfg: config.Config{}, settingsInput: textinput.New()}
}

func TestSettingsSetValidatesAndPersists(t *testing.T) {
	m := settingsTestModel(t)
	if err := settingsSet(&m.cfg, "download_rate_limit", "500K"); err != nil {
		t.Fatal(err)
	}
	if m.cfg.DownloadRateLimit != "500K" {
		t.Fatalf("rate = %q", m.cfg.DownloadRateLimit)
	}
	loaded := config.Load()
	if loaded.DownloadRateLimit != "500K" {
		t.Fatalf("persisted rate = %q", loaded.DownloadRateLimit)
	}
	before := m.cfg
	if err := settingsSet(&m.cfg, "download_rate_limit", "1T"); err == nil {
		t.Fatal("invalid rate unexpectedly accepted")
	}
	if m.cfg.DownloadRateLimit != before.DownloadRateLimit {
		t.Fatal("invalid value changed the active config")
	}
	if err := settingsSet(&m.cfg, "max_concurrent_downloads", "abc"); err == nil {
		t.Fatal("non-numeric value unexpectedly accepted")
	}
}

func TestSettingsToggleAndEdit(t *testing.T) {
	m := settingsTestModel(t)
	m.settingsView = true
	// Move to mouse_enabled (last field) and toggle it.
	for i := 1; i < len(settingsFields); i++ {
		updated, _ := m.handleSettingsKey("down", tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	updated, _ := m.handleSettingsKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.cfg.MouseEnabled {
		t.Fatal("bool toggle did not enable mouse")
	}
	// Editing a string field applies on Enter.
	for i := 0; i < len(settingsFields); i++ {
		updated, _ = m.handleSettingsKey("up", tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(Model)
	}
	updated, _ = m.handleSettingsKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.settingsEditing {
		t.Fatal("enter did not start editing a string field")
	}
	m.settingsInput.SetValue("dracula")
	updated, _ = m.handleSettingsKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.cfg.Theme != "dracula" || m.settingsEditing {
		t.Fatalf("edit result = theme %q, editing %v", m.cfg.Theme, m.settingsEditing)
	}
}

func TestSettingsCommandRanksFirst(t *testing.T) {
	m := settingsTestModel(t)
	m.commandPaletteInput.SetValue("settings")
	commands := m.filteredPaletteCommands()
	if len(commands) == 0 || commands[0].id != "settings" {
		t.Fatalf("first settings command = %#v, want general Settings", commands)
	}
}

func TestSettingsCommandOpensOverlay(t *testing.T) {
	m := settingsTestModel(t)
	m.commandPaletteInput.SetValue("settings")
	commands := m.filteredPaletteCommands()
	if len(commands) == 0 {
		t.Fatal("no settings command found")
	}
	updated, _ := m.executePaletteCommand("settings")
	if !updated.(Model).settingsView {
		t.Fatal("settings command did not open the overlay")
	}
}

func TestErrorMessageClearsOnNextKey(t *testing.T) {
	m := settingsTestModel(t)
	m.settingsView = true
	m.errMsg = "setting not saved: invalid value"
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).errMsg != "" {
		t.Fatalf("error message persisted after next key: %q", updated.(Model).errMsg)
	}
}

func TestSettingsContentShowsConfigPath(t *testing.T) {
	m := settingsTestModel(t)
	view := m.settingsContent(80)
	for _, want := range []string{"Settings", "Config:", "download_rate_limit", "Advanced options remain"} {
		if !strings.Contains(view, want) {
			t.Fatalf("settings content missing %q:\n%s", want, view)
		}
	}
}
