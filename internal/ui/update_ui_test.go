package ui

import (
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/update"
)

func TestUpdateCheckStatus(t *testing.T) {
	SetVersion("v0.0.1")
	m := Model{}
	updated, _ := m.handleUpdateCheck(updateCheckMsg{release: update.Release{
		TagName: "v0.1.0", HTMLURL: "https://example.com/release",
	}})
	status := updated.(Model).statusMsg
	if !strings.Contains(status, "v0.1.0") || !strings.Contains(status, "no automatic install") {
		t.Fatalf("status = %q", status)
	}
}

func TestTerminalPaletteChangeRepaintsOnlyTerminalTheme(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("default", nil) })
	m := Model{}
	if err := applyTheme("solarized-light", nil); err != nil {
		t.Fatal(err)
	}
	if _, cmd := m.Update(TerminalPaletteChangedMsg{}); cmd != nil {
		t.Fatal("fixed-color theme should ignore palette changes")
	}
	if err := applyTheme("terminal", nil); err != nil {
		t.Fatal(err)
	}
	if _, cmd := m.Update(TerminalPaletteChangedMsg{}); cmd == nil {
		t.Fatal("terminal theme should repaint on palette change")
	}
}

func TestAutoUpdateRespectsConfig(t *testing.T) {
	m := Model{cfg: configForUpdateTest(false, 0)}
	if cmd := m.maybeAutoUpdateCmd(); cmd != nil {
		t.Fatal("automatic check scheduled while disabled")
	}
}

func configForUpdateTest(enabled bool, hours int) (cfg config.Config) {
	cfg.UpdateCheckEnabled = enabled
	cfg.UpdateCheckIntervalHours = hours
	return
}
