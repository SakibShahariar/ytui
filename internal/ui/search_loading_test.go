package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/SakibShahariar/ytui/internal/store"
)

func TestSpinnerTicksWhileLoadingOnly(t *testing.T) {
	m := Model{loading: true}
	updated, cmd := m.Update(spinnerWakeMsg{})
	if cmd == nil || !updated.(Model).spinRunning {
		t.Fatal("loading did not start the spinner ticker")
	}
	updated, cmd = updated.(Model).Update(spinner.TickMsg{Time: time.Now()})
	if cmd == nil || !updated.(Model).spinRunning {
		t.Fatal("spinner ticker did not continue while loading")
	}
	idle := updated.(Model)
	idle.loading = false
	updated, cmd = idle.Update(spinner.TickMsg{Time: time.Now()})
	if updated.(Model).spinRunning || cmd == nil {
		t.Fatal("spinner ticker did not stop when loading ended")
	}
}

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
