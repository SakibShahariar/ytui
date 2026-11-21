package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// listFirstItemRow is the terminal row (0-based) of the first visible list
// item, derived from the same layout used by View(): header, panel border,
// any search/filter input lines, and the list title.
func (m Model) listFirstItemRow() int {
	row := headerLines + 1 // header + panel top border
	if m.activeTab == tabSearch || ((m.activeTab == tabHistory || m.activeTab == tabDownloads) &&
		(m.libraryFilterMode || m.libraryFilter != "")) {
		row += 2 // input line + divider
	}
	return row + 2 // list title + blank line before the first item
}

// handleMouse keeps mouse input additive: wheel events reuse the existing
// keyboard navigation paths, and a left click selects the item under the
// cursor. Overlays ignore clicks so they cannot change the hidden list.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	case tea.MouseButtonWheelDown:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if m.channelManager || m.rssViewer || m.settingsView || m.transcriptViewer || m.commandPalette || m.subtitlePicker ||
			m.sponsorBlockSettings || m.sponsorBlockStats || m.keyboardHelp ||
			m.downloadPicker || m.resumePrompt != nil || m.showLoginHelp ||
			m.logoutPending || m.downloadCancelPending != nil ||
			m.watchLaterBulkPending || m.watchLaterClearPending || m.offlineDeletePending != nil {
			return m, nil
		}
		row := m.listFirstItemRow()
		if msg.Y < row {
			return m, nil
		}
		// The list's own paginator knows the real page size (it subtracts
		// title/pagination rows before dividing by the delegate height).
		index := (msg.Y-row)/3 + m.list.Paginator.Page*m.list.Paginator.PerPage // delegate height 2 + spacing 1
		if index < 0 || index >= len(m.list.Items()) {
			return m, nil
		}
		m.list.Select(index)
		return m, m.refreshPreview()
	}
	return m, nil
}
