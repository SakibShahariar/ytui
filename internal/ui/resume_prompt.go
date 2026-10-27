package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) resumePromptContent(width int) string {
	if m.resumePrompt == nil {
		return ""
	}
	v := m.resumePrompt.video
	title := lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).MaxWidth(width).Render(v.Title)
	position := formatDuration(m.resumePrompt.position)
	return fmt.Sprintf("%s\n\nSaved progress: %s\n\n[Enter] Resume from %s\n[B] Start from beginning\n[Esc] Cancel",
		title, position, position)
}
