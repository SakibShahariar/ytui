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
