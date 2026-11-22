package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/SakibShahariar/ytui/internal/store"
)

func TestSearchLoadingIndicatorSitsNextToQuery(t *testing.T) {
	m := Model{
		activeTab:           tabSearch,
		inputMode:           true,
		loading:             true,
		width:               100,
		height:              30,
		searchInput:         textinput.New(),
		commandPaletteInput: textinput.New(),
		store:               &store.Store{},
	}
	m.searchInput.SetValue("gigguk")
	m.list = list.New(nil, ytDelegate{}, 60, 9)
	m.applyListSize()
	view := m.View()
	if !strings.Contains(view, "searching") {
		t.Fatal("loading indicator missing from search view")
	}
	line := strings.Split(view, "\n")[4]
	if index := strings.Index(line, "searching"); index < 0 || index > 40 {
		t.Fatalf("indicator not next to query (column %d): %q", index, line)
	}
}
