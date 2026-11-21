package ui

import (
	"testing"
)

func TestRemapKey(t *testing.T) {
	m := Model{}
	if got := m.remapKey("ctrl+c"); got != "ctrl+c" {
		t.Fatalf("default mapping = %q, want ctrl+c", got)
	}

	m.cfg.Keybindings = map[string]string{"quit": "ctrl+q"}
	if got := m.remapKey("ctrl+q"); got != "ctrl+c" {
		t.Fatalf("custom quit key = %q, want built-in ctrl+c", got)
	}
	if got := m.remapKey("ctrl+c"); got != "\x00" {
		t.Fatalf("old quit default = %q, want unbound sentinel", got)
	}
	if got := m.remapKey("ctrl+d"); got != "ctrl+d" {
		t.Fatalf("unrelated key = %q, want unchanged", got)
	}

	m.cfg.Keybindings = map[string]string{"download": "f5", "quit": "ctrl+q"}
	if got := m.remapKey("F5"); got != "ctrl+d" {
		t.Fatalf("uppercase custom key = %q, want ctrl+d", got)
	}
}
