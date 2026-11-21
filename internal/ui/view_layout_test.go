package ui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/store"
)

func TestPaneWidthsFloorForDegenerateTerminalWidths(t *testing.T) {
	for _, width := range []int{0, 1, 2, 3, 4, 8, 12} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			m := Model{width: width, height: 24, activeTab: tabSearch}
			listWidth, previewWidth := m.paneWidths()
			if listWidth < minPanelContentWidth {
				t.Fatalf("listWidth = %d, want >= %d", listWidth, minPanelContentWidth)
			}
			// View() subtracts four columns before passing the width to
			// content renderers that build dividers with strings.Repeat.
			if listWidth-4 < 0 {
				t.Fatalf("content width listWidth-4 = %d is negative", listWidth-4)
			}
			if previewWidth != 0 {
				t.Fatalf("previewWidth = %d, want 0 on a narrow terminal", previewWidth)
			}
		})
	}
}

func TestViewDoesNotPanicAtDegenerateWidth(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{name: "search", model: Model{activeTab: tabSearch, inputMode: true}},
		{name: "history filter", model: Model{activeTab: tabHistory, libraryFilter: "jazz"}},
		{name: "downloads filter", model: Model{activeTab: tabDownloads, libraryFilter: "test"}},
		{name: "keyboard help", model: Model{keyboardHelp: true}},
		{name: "command palette", model: Model{commandPalette: true}},
		{name: "login help", model: Model{showLoginHelp: true}},
		{name: "empty state", model: Model{activeTab: tabWatchLater, everSearched: true}},
	}

	for _, width := range []int{0, 1, 3} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s/width=%d", tt.name, width), func(t *testing.T) {
				m := tt.model
				m.width = width
				m.height = 24
				m.store = &store.Store{}
				m.list = list.New(nil, list.NewDefaultDelegate(), defaultWidth, defaultHeight)
				m.searchInput = textinput.New()
				m.commandPaletteInput = textinput.New()
				m.libraryFilterInput = textinput.New()
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("View() panicked: %v", r)
					}
				}()
				if got := m.View(); got == "" {
					t.Fatal("View() returned an empty frame")
				}
			})
		}
	}
}

func TestZeroWindowSizeMsgDoesNotPanic(t *testing.T) {
	m := Model{
		activeTab:   tabSearch,
		inputMode:   true,
		store:       &store.Store{},
		list:        list.New(nil, list.NewDefaultDelegate(), defaultWidth, defaultHeight),
		searchInput: textinput.New(),
		height:      defaultHeight,
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 0, Height: 24})
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() after zero-size WindowSizeMsg panicked: %v", r)
		}
	}()
	_ = result.View()
}
