package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchTranscript(t *testing.T) {
	text := "[00:01] Hello world\n[00:05] WORLD cup\n[00:09] other line"
	matches := searchTranscript(text, "WoRlD")
	if len(matches) != 2 || matches[0] != 0 || matches[1] != 1 {
		t.Fatalf("matches = %v, want [0 1]", matches)
	}
	if got := searchTranscript(text, ""); len(got) != 0 {
		t.Fatalf("empty query matches = %v, want none", got)
	}
}

func TestSearchTranscriptHandlesLongTranscripts(t *testing.T) {
	lines := make([]string, 20000)
	for i := range lines {
		lines[i] = "[00:12] regular caption line"
	}
	lines[19999] = "[12:34] the special needle"
	text := strings.Join(lines, "\n")
	start := time.Now()
	matches := searchTranscript(text, "needle")
	if time.Since(start) > 2*time.Second {
		t.Fatal("search took too long on a 20,000-line transcript")
	}
	if len(matches) != 1 || matches[0] != 19999 {
		t.Fatalf("matches = %v, want [19999]", matches)
	}
}

func TestTranscriptLineSeconds(t *testing.T) {
	for _, tt := range []struct {
		line string
		want float64
		ok   bool
	}{
		{"[00:12] caption", 12, true},
		{"[01:02:03] caption", 3723, true},
		{"caption", 0, false},
		{"[bad] caption", 0, false},
	} {
		got, ok := transcriptLineSeconds(tt.line)
		if got != tt.want || ok != tt.ok {
			t.Errorf("transcriptLineSeconds(%q) = %v, %v; want %v, %v", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTranscriptSearchNavigateAndJump(t *testing.T) {
	session := &fakePlaybackSession{}
	m := Model{
		transcriptViewer:      true,
		transcriptText:        "[00:01] first needle\n[01:02] SECOND Needle\n[02:00] last line",
		transcriptSearchInput: textinput.New(),
		nowPlaying:            session,
	}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if !m.transcriptSearchMode {
		t.Fatal("/ did not enter transcript search mode")
	}
	for _, r := range "needle" {
		updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.transcriptSearchMode || len(m.transcriptMatches) != 2 {
		t.Fatalf("search state = mode %v, matches %v; want 2 matches", m.transcriptSearchMode, m.transcriptMatches)
	}
	if m.transcriptOffset != 0 {
		t.Fatalf("offset = %d after first match, want 0", m.transcriptOffset)
	}

	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	if m.transcriptMatchIndex != 1 || m.transcriptOffset != 1 {
		t.Fatalf("next match index = %d, offset = %d; want 1, 1", m.transcriptMatchIndex, m.transcriptOffset)
	}

	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if !session.seekCalled || session.seekTo != 62 {
		t.Fatalf("jump seek = %v, %v; want 62s", session.seekCalled, session.seekTo)
	}
}
