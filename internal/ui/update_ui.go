package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/update"
)

// appVersion is set by main so update messages can compare versions without
// importing main's package-level version flag.
var appVersion = "dev"

func SetVersion(version string) { appVersion = version }

// TerminalPaletteChangedMsg asks the UI to repaint using the terminal's
// current ANSI palette. Send it after a theme generator (e.g. matugen)
// changes the terminal colors; it is a no-op for fixed-color themes.
type TerminalPaletteChangedMsg struct{}

type updateCheckMsg struct {
	release update.Release
	err     error
}

func checkUpdatesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		release, err := update.Check(ctx, nil, update.DefaultEndpoint)
		return updateCheckMsg{release: release, err: err}
	}
}

// maybeAutoUpdateCmd schedules an automatic check only when enabled and the
// configured interval has elapsed.
func (m Model) maybeAutoUpdateCmd() tea.Cmd {
	if !m.cfg.UpdateCheckEnabled {
		return nil
	}
	interval := time.Duration(m.cfg.UpdateCheckIntervalHours) * time.Hour
	if !update.ShouldCheck(update.CachePath(), interval, time.Now()) {
		return nil
	}
	return checkUpdatesCmd()
}

func (m Model) handleUpdateCheck(msg updateCheckMsg) (tea.Model, tea.Cmd) {
	if errors.Is(msg.err, update.ErrNoReleases) {
		m.statusMsg = "no releases published yet"
		return m, nil
	}
	if msg.err != nil {
		m.statusMsg = "update check failed: " + msg.err.Error()
		return m, nil
	}
	_ = update.RecordCheck(update.CachePath(), time.Now())
	if update.NewerThan(appVersion, msg.release.TagName) {
		m.statusMsg = fmt.Sprintf("update %s available — %s (no automatic install)", msg.release.TagName, msg.release.HTMLURL)
	} else {
		m.statusMsg = "ytui is up to date (" + msg.release.TagName + ")"
	}
	return m, nil
}
