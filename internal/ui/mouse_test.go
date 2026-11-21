package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func mouseTestModel() Model {
	items := []list.Item{
		videoItem{v: ytdlp.Video{ID: "a", Title: "First", URL: "https://example.com/a"}},
		videoItem{v: ytdlp.Video{ID: "b", Title: "Second", URL: "https://example.com/b"}},
		videoItem{v: ytdlp.Video{ID: "c", Title: "Third", URL: "https://example.com/c"}},
	}
	m := Model{activeTab: tabWatchLater}
	m.list = list.New(items, ytDelegate{}, 60, 9)
	m.list.Title = "Results"
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.DisableQuitKeybindings()
	m.list.Select(0)
	return m
}

func TestMouseWheelMovesSelection(t *testing.T) {
	m := mouseTestModel()
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if got := updated.(Model).list.Index(); got != 1 {
		t.Fatalf("wheel down index = %d, want 1", got)
	}
	updated, _ = updated.(Model).Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if got := updated.(Model).list.Index(); got != 0 {
		t.Fatalf("wheel up index = %d, want 0", got)
	}
}

func TestMouseClickSelectsItemUnderCursor(t *testing.T) {
	m := mouseTestModel()
	row := m.listFirstItemRow()
	// Each item occupies three terminal rows (two content lines + spacing).
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: row + 6})
	if got := updated.(Model).list.Index(); got != 2 {
		t.Fatalf("click index = %d, want 2", got)
	}
	// Clicks above the list are ignored.
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: row - 1})
	if got := updated.(Model).list.Index(); got != 0 {
		t.Fatalf("click above list changed selection to %d", got)
	}
}

func TestListFirstItemRowMatchesRenderedView(t *testing.T) {
	for _, tt := range []struct {
		name string
		tab  tab
	}{
		{name: "watch later", tab: tabWatchLater},
		{name: "search", tab: tabSearch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := mouseTestModel()
			m.activeTab = tt.tab
			m.width, m.height = 100, 30
			m.applyListSize()
			view := m.View()
			for i, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "First") {
					if got := m.listFirstItemRow(); got != i {
						t.Fatalf("listFirstItemRow = %d, rendered first item at row %d", got, i)
					}
					return
				}
			}
			t.Fatal("first item not found in rendered view")
		})
	}
}

func TestMouseClickIgnoredInOverlay(t *testing.T) {
	m := mouseTestModel()
	m.helpKeyboardsForTest()
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: m.listFirstItemRow() + 2})
	if got := updated.(Model).list.Index(); got != 0 {
		t.Fatalf("click in overlay changed selection to %d", got)
	}
}

// helpKeyboardsForTest puts the model into an overlay state without needing
// a full key event round trip.
func (m *Model) helpKeyboardsForTest() {
	m.keyboardHelp = true
}
