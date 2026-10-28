package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// keyboardHelpLines is the in-app reference for keys handled by handleKey.
// Plain letters are deliberately omitted because they type into Search.
func keyboardHelpLines() []string {
	return []string{
		"Navigation",
		"Tab / Shift+Tab     Switch tabs",
		"↑ / ↓               Move through lists",
		"Enter               Search or open/play selection",
		"/                   Jump to Search",
		"?                   Open/close this help",
		"Esc                 Cancel search, close help, or back out",
		"",
		"Search and lists",
		"Plain letters       Start or edit a Search query",
		"Ctrl+S              Subscribe to selected channel",
		"Ctrl+W              Add selected video to Watch Later",
		"Watch Later: Space   Select/deselect an item",
		"Watch Later: m       Remove selected items (confirmation required)",
		"Watch Later: s       Cycle sort: added, title, duration, channel",
		"Watch Later: x       Clear all items (confirmation required)",
		"Ctrl+D              Open download options; cancel selected download",
		"Ctrl+R              Retry/resume selected failed download",
		"Ctrl+Space          Pause/resume selected download",
		"Ctrl+↑ / Ctrl+↓     Move queued download up/down",
		"Ctrl+O              Toggle download queue / offline files",
		"Delete              Remove selected offline file (asks for confirmation)",
		"",
		"Playback",
		"Ctrl+A              Toggle audio-only mode",
		"Ctrl+P              Pause or resume playback",
		"Ctrl+N              Toggle autoplay next (saved preference)",
		"Ctrl+X              Stop playback",
		"[ / ]               Seek backward / forward 10 seconds",
		"- / =               Decrease / increase volume",
		"Ctrl+T              Toggle thumbnail previews",
		"",
		"Account and exit",
		"Ctrl+L              Log in or out",
		"Ctrl+C              Quit ytui",
	}
}

func (m Model) keyboardHelpMaxOffset() int {
	visibleRows := m.panelHeight() - 5 // one header row plus panel borders/search header
	if visibleRows < 1 { visibleRows = 1 }
	maxOffset := len(keyboardHelpLines()) - visibleRows
	if maxOffset < 0 { return 0 }
	return maxOffset
}

func (m Model) keyboardHelpContent(width, height int) string {
	lines := keyboardHelpLines()
	if height < 2 { height = 2 }
	visibleRows := height - 1 // reserve one row for the help title
	maxOffset := len(lines) - visibleRows
	if maxOffset < 0 { maxOffset = 0 }
	offset := m.keyboardHelpOffset
	if offset < 0 { offset = 0 }
	if offset > maxOffset { offset = maxOffset }
	end := offset + visibleRows
	if end > len(lines) { end = len(lines) }
	visible := make([]string, 0, end-offset)
	for _, line := range lines[offset:end] {
		switch line {
		case "Navigation", "Search and lists", "Playback", "Account and exit":
			visible = append(visible, lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render(line))
		default:
			visible = append(visible, lipgloss.NewStyle().MaxWidth(width).Render(line))
		}
	}
	header := fmt.Sprintf("Keyboard help (%d–%d of %d; use ↑/↓ or PgUp/PgDn)", offset+1, end, len(lines))
	return lipgloss.NewStyle().Width(width).Render(header) + "\n" + strings.Join(visible, "\n")
}
