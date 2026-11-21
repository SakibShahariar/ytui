package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/store"
)

// recordSponsorBlockSkip updates the local, private skip counters. Failures
// are surfaced but never interrupt playback.
func (m *Model) recordSponsorBlockSkip(category string, seconds float64) {
	if m.store == nil {
		return
	}
	if err := m.store.RecordSponsorBlockSkip(category, seconds); err != nil {
		m.errMsg = "save SponsorBlock statistics: " + err.Error()
	}
}

func (m Model) sponsorBlockStatsContent(width int) string {
	if width < 12 {
		width = 12
	}
	var skips int
	var seconds float64
	var byCategory []store.SponsorBlockStat
	if m.store != nil {
		skips, seconds, byCategory = m.store.SponsorBlockSummary()
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("SponsorBlock statistics"),
		"",
		fmt.Sprintf("Total: %d skips · about %s saved", skips, formatDuration(seconds)),
		"",
	}
	if len(byCategory) == 0 {
		lines = append(lines, "No segments skipped yet.")
	} else {
		for _, stat := range byCategory {
			lines = append(lines, fmt.Sprintf("  %-16s %4d skips  %8s", stat.Category, stat.Skips, formatDuration(stat.SecondsSaved)))
		}
	}
	lines = append(lines, "", "r: reset statistics · esc: close")
	return strings.Join(lines, "\n")
}

func (m Model) resetSponsorBlockStats() (tea.Model, tea.Cmd) {
	if m.store == nil {
		return m, nil
	}
	if err := m.store.ResetSponsorBlockStats(); err != nil {
		m.errMsg = "reset SponsorBlock statistics: " + err.Error()
	} else {
		m.statusMsg = "SponsorBlock statistics reset"
	}
	return m, nil
}
