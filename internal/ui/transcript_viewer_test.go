package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTranscriptViewerEscapeClosesWithoutChangingPlayback(t *testing.T) {
	m := Model{
		transcriptViewer: true,
		transcriptTitle: "A video",
		transcriptText: "first line\nsecond line",
		transcriptOffset: 1,
		playingVid: nil,
	}
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.transcriptViewer { t.Fatal("Escape left transcript viewer open") }
	if got.transcriptText != "" || got.transcriptTitle != "" { t.Fatal("closing transcript did not clear viewer-only content") }
	if cmd != nil { t.Fatalf("Escape returned unexpected command: %v", cmd) }
}

func TestTranscriptViewerScrollClampsAtStart(t *testing.T) {
	m := Model{transcriptViewer: true, transcriptText: "one\ntwo\nthree", transcriptOffset: 0}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if got := updated.(Model).transcriptOffset; got != 0 { t.Fatalf("offset = %d after scrolling above start, want 0", got) }
}
