package ui

import (
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/config"
)

func TestApplyThemeChangesPaletteAndStyles(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("default", nil) })
	if err := applyTheme("high-contrast", nil); err != nil {
		t.Fatal(err)
	}
	if string(ctpBase) != "#000000" || string(ctpMauve) != "#ffff00" {
		t.Fatalf("palette = base %s, mauve %s", ctpBase, ctpMauve)
	}
	if !strings.Contains(titleStyle.Render("ytui"), "ytui") {
		t.Fatal("styled title lost its text")
	}
	if activeTheme != "high-contrast" {
		t.Fatalf("activeTheme = %q", activeTheme)
	}
}

func TestTerminalThemeUsesANSIPalette(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("default", nil) })
	if err := applyTheme("terminal", nil); err != nil {
		t.Fatal(err)
	}
	if string(ctpMauve) != "5" || string(ctpBase) != "0" || string(ctpText) != "7" {
		t.Fatalf("terminal palette = base %s, text %s, mauve %s", ctpBase, ctpText, ctpMauve)
	}
}

func TestApplyThemeColorOverrides(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("default", nil) })
	if err := applyTheme("minimal", map[string]string{"mauve": "#ff00ff"}); err != nil {
		t.Fatal(err)
	}
	if string(ctpMauve) != "#ff00ff" {
		t.Fatalf("override mauve = %s", ctpMauve)
	}
}

func TestThemeAvailableInCommandPalette(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("default", nil) })
	m := Model{cfg: config.Config{Theme: "default"}}
	m.commandPaletteInput.SetValue("theme:")
	commands := m.filteredPaletteCommands()
	if len(commands) != len(config.BuiltinThemes) {
		t.Fatalf("theme commands = %d, want %d", len(commands), len(config.BuiltinThemes))
	}
	m.cfg.Colors = map[string]string{"mauve": "#ff00ff"}
	updated, _ := m.executePaletteCommand("theme:light")
	result := updated.(Model)
	if result.cfg.Theme != "light" || activeTheme != "light" {
		t.Fatalf("theme not switched: cfg %q, active %q", result.cfg.Theme, activeTheme)
	}
	if len(result.cfg.Colors) != 0 || string(ctpMauve) == "#ff00ff" {
		t.Fatalf("custom colors survived built-in theme switch: %v, mauve %s", result.cfg.Colors, ctpMauve)
	}
}
