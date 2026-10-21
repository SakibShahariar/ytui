package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"ytui/internal/library"
	"ytui/internal/ytdlp"
)

// videoItem adapts ytdlp.Video to the list.Item interface bubbles/list needs.
// ytDelegate is a custom list.ItemDelegate replacing bubbles/list's
// DefaultDelegate. DefaultDelegate's internal title/description truncation
// assumes rune-count roughly equals terminal display width, which breaks
// down for scripts like Bengali, Devanagari, and Thai where combining
// marks and conjuncts make multiple Unicode codepoints render as a single,
// narrower glyph — that mismatch was producing garbled output (misaligned
// text, stray leftover border/padding characters) for that kind of
// content. This delegate does its own truncation via uniseg instead.
type ytDelegate struct{}

func (ytDelegate) Height() int                       { return 2 }
func (ytDelegate) Spacing() int                       { return 1 }
func (ytDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (ytDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	type titler interface{ Title() string }
	type describer interface{ Description() string }

	var title, desc string
	if t, ok := item.(titler); ok {
		title = t.Title()
	}
	if d, ok := item.(describer); ok {
		desc = d.Description()
	}

	innerWidth := m.Width() - 4 // selection bar + padding
	if innerWidth < 4 {
		innerWidth = 4
	}
	title = truncate(title, innerWidth)
	desc = truncate(desc, innerWidth)

	selected := index == m.Index()

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ctpMauve).PaddingLeft(1)
		descStyle = lipgloss.NewStyle().Foreground(ctpPink).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ctpMauve).PaddingLeft(1)
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(ctpText).PaddingLeft(2)
		descStyle = lipgloss.NewStyle().Foreground(ctpSubtext0).PaddingLeft(2)
	}

	fmt.Fprint(w, titleStyle.Render(title)+"\n"+descStyle.Render(desc))
}

type videoItem struct{ v ytdlp.Video }

type localDownloadItem struct{ file library.File }

func (i localDownloadItem) Title() string { return i.file.Name }
func (i localDownloadItem) Description() string {
	return fmt.Sprintf("%s · modified %s", formatBytes(i.file.Size), i.file.ModTime.Format("Jan 2 15:04"))
}
func (i localDownloadItem) FilterValue() string { return i.file.Name }

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit { return fmt.Sprintf("%d B", n) }
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit { div *= unit; exp++ }
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (i videoItem) Title() string {
	return fmt.Sprintf("%s  [%s]", i.v.Title, formatDuration(i.v.Duration))
}
func (i videoItem) Description() string {
	return fmt.Sprintf("%s · %s views", i.v.Channel, formatViews(i.v.ViewCount))
}
func (i videoItem) FilterValue() string { return i.v.Title }

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatViews(v int64) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}

// truncate shortens s to at most width runes, appending an ellipsis if it
// had to cut anything. width <= 1 just returns "…".
// truncate shortens s to at most `width` terminal display columns,
// appending an ellipsis if it had to cut anything. Uses uniseg for actual
// grapheme-cluster-aware width, not a naive rune count — scripts like
// Bengali, Devanagari, and Thai combine multiple Unicode codepoints
// (base character + combining vowel signs / conjuncts) into a single
// visual glyph that occupies far fewer terminal columns than its rune
// count would suggest. Counting runes instead of display width is what
// caused garbled, misaligned rendering (stray leftover characters) for
// that kind of text. width <= 1 just returns "…".
func truncate(s string, width int) string {
	if uniseg.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	var b strings.Builder
	col := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		w := uniseg.StringWidth(gr.Str())
		if col+w > width-1 { // leave room for the ellipsis
			break
		}
		b.WriteString(gr.Str())
		col += w
	}
	return b.String() + "…"
}

// wrapTwoLines word-wraps s to fit width, returning at most 2 lines. If
// wrapping produces more than 2 lines, the second line is ellipsized so
// nothing overflows past it — used for the preview panel's video title,
// which previously got hard-truncated to a single line even when there
// was room to show more of it.
func wrapTwoLines(s string, width int) (line1, line2 string) {
	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > 0 {
		line1 = lines[0]
	}
	if len(lines) > 1 {
		line2 = lines[1]
	}
	if len(lines) > 2 {
		line2 = truncate(line2, width)
	}
	return line1, line2
}

