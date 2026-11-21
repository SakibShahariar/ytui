package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/store"
)

func statsTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return Model{store: s, searchInput: textinput.New(), commandPaletteInput: textinput.New()}
}

func TestSponsorBlockStatsRecordAndDisplay(t *testing.T) {
	m := statsTestModel(t)
	m.recordSponsorBlockSkip("sponsor", 12.5)
	m.recordSponsorBlockSkip("sponsor", 2.5)
	m.recordSponsorBlockSkip("selfpromo", 3)
	view := m.sponsorBlockStatsContent(60)
	for _, want := range []string{"3 skips", "0:15", "selfpromo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("stats view missing %q:\n%s", want, view)
		}
	}
}

func TestSponsorBlockStatsResetAndClose(t *testing.T) {
	m := statsTestModel(t)
	m.recordSponsorBlockSkip("sponsor", 10)
	m.sponsorBlockStats = true
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)
	if !strings.Contains(m.statusMsg, "reset") {
		t.Fatalf("reset status = %q", m.statusMsg)
	}
	skips, seconds, _ := m.store.SponsorBlockSummary()
	if skips != 0 || seconds != 0 {
		t.Fatalf("stats after reset = %d, %v", skips, seconds)
	}
	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).sponsorBlockStats {
		t.Fatal("esc did not close stats overlay")
	}
}
