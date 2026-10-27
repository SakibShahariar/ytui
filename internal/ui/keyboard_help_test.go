package ui

import (
	"strings"
	"testing"
)

func TestKeyboardHelpListsRealControlBindings(t *testing.T) {
	got := strings.Join(keyboardHelpLines(), "\n")
	for _, want := range []string{
		"Tab / Shift+Tab", "Ctrl+S", "Ctrl+W", "Ctrl+D", "Ctrl+R",
		"Ctrl+O", "Ctrl+A", "Ctrl+P", "Ctrl+X", "Ctrl+T", "Ctrl+L", "Ctrl+C",
		"Plain letters",
	} {
		if !strings.Contains(got, want) { t.Errorf("keyboard help missing %q", want) }
	}
	for _, stale := range []string{"press s to subscribe", "press w to watch later", "press p to pause"} {
		if strings.Contains(strings.ToLower(got), stale) { t.Errorf("keyboard help contains stale binding %q", stale) }
	}
}

func TestKeyboardHelpContentScrollsAndClamps(t *testing.T) {
	m := Model{keyboardHelpOffset: 4}
	got := m.keyboardHelpContent(60, 5)
	if !strings.Contains(got, "Search and lists") { t.Fatalf("expected scrolled help section, got:\n%s", got) }
	m.keyboardHelpOffset = 1000
	got = m.keyboardHelpContent(60, 5)
	if !strings.Contains(got, "Ctrl+C") { t.Fatalf("end-clamped help should show final binding, got:\n%s", got) }
}
