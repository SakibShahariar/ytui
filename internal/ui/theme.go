package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/config"
)

// palette holds the semantic color roles used throughout the UI.
type palette struct {
	base, mantle, text, subtext1, subtext0, overlay1, overlay0                 lipgloss.Color
	surface2, surface1, mauve, pink, red, peach, yellow, green, blue, lavender lipgloss.Color
}

var builtinPalettes = map[string]palette{
	"default": {
		base: "#1e1e2e", mantle: "#181825", text: "#cdd6f4", subtext1: "#bac2de",
		subtext0: "#a6adc8", overlay1: "#7f849c", overlay0: "#6c7086",
		surface2: "#585b70", surface1: "#45475a", mauve: "#cba6f7",
		pink: "#f5c2e7", red: "#f38ba8", peach: "#fab387", yellow: "#f9e2af",
		green: "#a6e3a1", blue: "#89b4fa", lavender: "#b4befe",
	},
	"minimal": {
		base: "#111111", mantle: "#191919", text: "#dddddd", subtext1: "#bbbbbb",
		subtext0: "#999999", overlay1: "#777777", overlay0: "#666666",
		surface2: "#555555", surface1: "#444444", mauve: "#cccccc",
		pink: "#dddddd", red: "#ff6666", peach: "#ffaa66", yellow: "#ffcc66",
		green: "#88cc88", blue: "#88aaff", lavender: "#bbbbff",
	},
	"high-contrast": {
		base: "#000000", mantle: "#000000", text: "#ffffff", subtext1: "#ffffff",
		subtext0: "#dddddd", overlay1: "#aaaaaa", overlay0: "#888888",
		surface2: "#555555", surface1: "#444444", mauve: "#ffff00",
		pink: "#ff66ff", red: "#ff4444", peach: "#ff8800", yellow: "#ffff00",
		green: "#00ff00", blue: "#00aaff", lavender: "#aaaaff",
	},
	"light": {
		base: "#fdf6e3", mantle: "#eee8d5", text: "#073642", subtext1: "#586e75",
		subtext0: "#657b83", overlay1: "#839496", overlay0: "#93a1a1",
		surface2: "#d3d7c8", surface1: "#b8bdad", mauve: "#6c71c4",
		pink: "#d33682", red: "#dc322f", peach: "#cb4b16", yellow: "#b58900",
		green: "#859900", blue: "#268bd2", lavender: "#6c71c4",
	},
	// terminal defers to the terminal emulator's own ANSI palette: indices
	// 0-7 are the normal colors and 8-15 the bright ones. Changing your
	// terminal color scheme changes ytui without touching config.json.
	"terminal": {
		base: "0", mantle: "8", text: "15", subtext1: "7",
		subtext0: "8", overlay1: "8", overlay0: "8",
		surface2: "8", surface1: "8", mauve: "13",
		pink: "13", red: "9", peach: "11", yellow: "11",
		green: "10", blue: "12", lavender: "13",
	},
}

// applyTheme installs a palette plus optional per-role overrides and rebuilds
// the package-level styles that capture colors at construction time. Other
// styles read the role variables directly while rendering.
func applyTheme(name string, overrides map[string]string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "default"
	}
	base, ok := builtinPalettes[name]
	if !ok {
		return fmt.Errorf("unknown theme %q", name)
	}
	for role, value := range overrides {
		role = strings.ToLower(strings.TrimSpace(role))
		roleColors := map[string]*lipgloss.Color{
			"base": &base.base, "mantle": &base.mantle, "text": &base.text,
			"subtext1": &base.subtext1, "subtext0": &base.subtext0,
			"overlay1": &base.overlay1, "overlay0": &base.overlay0,
			"surface2": &base.surface2, "surface1": &base.surface1,
			"mauve": &base.mauve, "pink": &base.pink, "red": &base.red,
			"peach": &base.peach, "yellow": &base.yellow, "green": &base.green,
			"blue": &base.blue, "lavender": &base.lavender,
		}
		target, found := roleColors[role]
		if !found {
			return fmt.Errorf("unknown palette role %q", role)
		}
		*target = lipgloss.Color(value)
	}

	ctpBase, ctpMantle, ctpText = base.base, base.mantle, base.text
	ctpSubtext1, ctpSubtext0 = base.subtext1, base.subtext0
	ctpOverlay1, ctpOverlay0 = base.overlay1, base.overlay0
	ctpSurface2, ctpSurface1 = base.surface2, base.surface1
	ctpMauve, ctpPink, ctpRed = base.mauve, base.pink, base.red
	ctpPeach, ctpYellow, ctpGreen = base.peach, base.yellow, base.green
	ctpBlue, ctpLavender = base.blue, base.lavender

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(ctpMauve)
	subtitleStyle = lipgloss.NewStyle().Foreground(ctpOverlay1).Italic(true)
	dividerStyle = lipgloss.NewStyle().Foreground(ctpSurface1)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Background(ctpMauve).Foreground(ctpBase).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(ctpOverlay0).Padding(0, 1)
	errorStyle = lipgloss.NewStyle().Foreground(ctpRed).Bold(true)
	panelBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ctpSurface1)
	previewLabel = lipgloss.NewStyle().Foreground(ctpLavender).Bold(true)
	previewMutedText = lipgloss.NewStyle().Foreground(ctpOverlay0).Align(lipgloss.Center)
	footerBarStyle = lipgloss.NewStyle().Background(ctpMantle).Foreground(ctpSubtext0)
	footerModeNav = lipgloss.NewStyle().Background(ctpBlue).Foreground(ctpBase).Bold(true).Padding(0, 1)
	footerModeInput = lipgloss.NewStyle().Background(ctpGreen).Foreground(ctpBase).Bold(true).Padding(0, 1)
	activeTheme = name
	return nil
}

var activeTheme = "default"

func (m Model) applyConfiguredTheme() {
	if err := applyTheme(m.cfg.Theme, m.cfg.Colors); err != nil {
		m.errMsg = "theme: " + err.Error()
	}
}

// themeNames returns the selectable themes for the command palette.
func themeNames() []string { return config.BuiltinThemes }
