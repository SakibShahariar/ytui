package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
)

func TestColonOpensCommandPaletteAndEscapeClosesIt(t *testing.T) {
	m := Model{
		activeTab: tabHistory,
		searchInput: textinput.New(),
		commandPaletteInput: textinput.New(),
	}
	opened, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	palette := opened.(Model)
	if !palette.commandPalette { t.Fatal("colon did not open command palette") }
	closed, _ := palette.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if closed.(Model).commandPalette { t.Fatal("Escape did not close command palette") }
}

func TestCommandPaletteAudioActionTogglesAndCloses(t *testing.T) {
	m := Model{commandPalette: true, commandPaletteInput: textinput.New(), searchInput: textinput.New()}
	result, _ := m.executePaletteCommand("toggle_audio")
	got := result.(Model)
	if !got.audioOnly { t.Fatal("audio-only mode was not enabled") }
	if got.commandPalette { t.Fatal("palette remained open after executing command") }
}

func TestCommandPaletteEmptyResultsKeepCursorValid(t *testing.T) {
	m := Model{commandPalette: true, commandPaletteInput: textinput.New(), searchInput: textinput.New()}
	m.commandPaletteInput.SetValue("no-such-command")
	for _, key := range []string{"pgdown", "down", "end", "pgup", "up"} {
		updated, _ := m.handleKey(tea.KeyMsg{Type: map[string]tea.KeyType{
			"pgdown": tea.KeyPgDown, "down": tea.KeyDown, "end": tea.KeyEnd,
			"pgup": tea.KeyPgUp, "up": tea.KeyUp,
		}[key]})
		m = updated.(Model)
		if m.commandPaletteCursor != 0 {
			t.Fatalf("cursor after %s = %d with no matches, want 0", key, m.commandPaletteCursor)
		}
	}
	if _, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}); m.commandPaletteCursor != 0 {
		t.Fatalf("cursor after empty-result Enter = %d, want 0", m.commandPaletteCursor)
	}
}
