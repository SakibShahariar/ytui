package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/config"
)

// palette holds the semantic color roles used throughout the UI.
type palette struct {
	base, mantle, text, subtext1, subtext0, overlay1, overlay0                 lipgloss.Color
	surface2, surface1, mauve, pink, red, peach, yellow, green, blue, lavender lipgloss.Color
}

var builtinPalettes = map[string]palette{
	// terminal defers to the terminal emulator's own ANSI palette. Use the
	// normal colors (0-7) rather than bright variants: they follow the
	// terminal's color scheme without the garish look of indices 8-15.
	"terminal": {
		base: "0", mantle: "0", text: "7", subtext1: "7",
		subtext0: "7", overlay1: "8", overlay0: "8",
		surface2: "8", surface1: "8", mauve: "5",
		pink: "5", red: "1", peach: "3", yellow: "3",
		green: "2", blue: "4", lavender: "5",
	},
	"catppuccin-mocha": {
		base: "#1e1e2e", mantle: "#181825", text: "#cdd6f4", subtext1: "#bac2de",
		subtext0: "#a6adc8", overlay1: "#7f849c", overlay0: "#6c7086",
		surface2: "#585b70", surface1: "#45475a", mauve: "#cba6f7",
		pink: "#f5c2e7", red: "#f38ba8", peach: "#fab387", yellow: "#f9e2af",
		green: "#a6e3a1", blue: "#89b4fa", lavender: "#b4befe",
	},
	"nord": {
		base: "#2e3440", mantle: "#292e39", text: "#eceff4", subtext1: "#d8dee9",
		subtext0: "#b8c0cc", overlay1: "#8f9bb3", overlay0: "#7b88a1",
		surface2: "#5e687d", surface1: "#4c566a", mauve: "#b48ead",
		pink: "#b48ead", red: "#bf616a", peach: "#d08770", yellow: "#ebcb8b",
		green: "#a3be8c", blue: "#88c0d0", lavender: "#81a1c1",
	},
	"gruvbox-dark": {
		base: "#282828", mantle: "#1d2021", text: "#ebdbb2", subtext1: "#d5c4a1",
		subtext0: "#bdae93", overlay1: "#a89984", overlay0: "#928374",
		surface2: "#665c54", surface1: "#504945", mauve: "#b16286",
		pink: "#b16286", red: "#fb4934", peach: "#fe8019", yellow: "#fabd2f",
		green: "#b8bb26", blue: "#83a598", lavender: "#d3869b",
	},
	"dracula": {
		base: "#282a36", mantle: "#21222c", text: "#f8f8f2", subtext1: "#e2e2e3",
		subtext0: "#c3c5ca", overlay1: "#a4a7b5", overlay0: "#8b8e9f",
		surface2: "#6272a4", surface1: "#44475a", mauve: "#bd93f9",
		pink: "#ff79c6", red: "#ff5555", peach: "#ffb86c", yellow: "#f1fa8c",
		green: "#50fa7b", blue: "#8be9fd", lavender: "#bd93f9",
	},
	"solarized-dark": {
		base: "#002b36", mantle: "#073642", text: "#eee8d5", subtext1: "#93a1a1",
		subtext0: "#839496", overlay1: "#657b83", overlay0: "#586e75",
		surface2: "#073642", surface1: "#002b36", mauve: "#6c71c4",
		pink: "#d33682", red: "#dc322f", peach: "#cb4b16", yellow: "#b58900",
		green: "#859900", blue: "#268bd2", lavender: "#6c71c4",
	},
	"solarized-light": {
		base: "#fdf6e3", mantle: "#eee8d5", text: "#073642", subtext1: "#586e75",
		subtext0: "#657b83", overlay1: "#839496", overlay0: "#93a1a1",
		surface2: "#d3d7c8", surface1: "#b8bdad", mauve: "#6c71c4",
		pink: "#d33682", red: "#dc322f", peach: "#cb4b16", yellow: "#b58900",
		green: "#859900", blue: "#268bd2", lavender: "#6c71c4",
	},
	"tokyo-night": {
		base: "#1a1b26", mantle: "#16161e", text: "#c0caf5", subtext1: "#a9b1dc",
		subtext0: "#828bb8", overlay1: "#6a7194", overlay0: "#565589",
		surface2: "#4b526d", surface1: "#3b4261", mauve: "#bb9af7",
		pink: "#f0a9e0", red: "#f7768e", peach: "#ff9e64", yellow: "#e0af68",
		green: "#9ece6a", blue: "#7aa2f7", lavender: "#bb9af7",
	},
	"rose-pine": {
		base: "#191724", mantle: "#1f1d2e", text: "#e0def4", subtext1: "#c4c0e0",
		subtext0: "#a6a0c4", overlay1: "#908caa", overlay0: "#6e6a86",
		surface2: "#403d52", surface1: "#26233a", mauve: "#c4a7e7",
		pink: "#ebbcba", red: "#eb6f92", peach: "#f6c177", yellow: "#f6c177",
		green: "#9ccfd8", blue: "#9ccfd8", lavender: "#c4a7e7",
	},
	"one-dark": {
		base: "#282c34", mantle: "#21252b", text: "#abb2bf", subtext1: "#9da3ad",
		subtext0: "#848b98", overlay1: "#6b7280", overlay0: "#565c66",
		surface2: "#4b5263", surface1: "#3f4451", mauve: "#c678dd",
		pink: "#d19a66", red: "#e06c75", peach: "#d19a66", yellow: "#e5c07b",
		green: "#98c379", blue: "#61afef", lavender: "#c678dd",
	},
}

// applyTheme installs a palette plus optional per-role overrides and rebuilds
// the package-level styles that capture colors at construction time. Other
// styles read the role variables directly while rendering.
func applyTheme(name string, overrides map[string]string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "terminal"
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

var activeTheme = "terminal"

// restyleComponents rebuilds styles that were captured when the text inputs
// and list were created. Without this, a live theme switch would leave those
// widgets using the previous palette.
func (m *Model) restyleComponents() {
	styleInput := func(input *textinput.Model) {
		input.PromptStyle = lipgloss.NewStyle().Foreground(ctpMauve)
		input.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
		input.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
		input.Cursor.Style = lipgloss.NewStyle().Foreground(ctpMauve)
	}
	styleInput(&m.searchInput)
	styleInput(&m.commandPaletteInput)
	styleInput(&m.libraryFilterInput)
	styleInput(&m.pathInput)
	styleInput(&m.transcriptSearchInput)
	styleInput(&m.settingsInput)
	styleInput(&m.channelGroupInput)
	m.list.Styles.Title = lipgloss.NewStyle().Background(ctpMauve).Foreground(ctpBase).Bold(true).Padding(0, 1)
}

func (m *Model) applyConfiguredTheme() {
	if err := applyTheme(m.cfg.Theme, m.cfg.Colors); err != nil {
		m.errMsg = "theme: " + err.Error()
	}
	m.restyleComponents()
}

// themeNames returns the selectable themes for the command palette.
func themeNames() []string { return config.BuiltinThemes }
