package ui

import (
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"ytui/internal/thumb"
)

// nowPlayingImageRegion computes where the Now Playing tab's (much larger,
// since it doesn't share space with a results list) thumbnail draws.
func (m Model) nowPlayingImageRegion() (col, row, cols, rows int) {
	listWidth, _ := m.paneWidths() // previewVisible() is false for this tab (no list items), so this is the full content width
	col = 3                        // panel border + padding
	row = headerLines + 2          // header rows + panel top border + a blank line

	cols = listWidth - 6
	if cols > 90 {
		cols = 90 // cap — an enormous thumbnail on an ultrawide monitor looks worse, not better
	}
	if cols < 20 {
		cols = 20
	}

	rows = int(math.Round(float64(cols) * 9.0 / 32.0)) // 16:9 aspect, terminal cells ~2x taller than wide
	maxRows := m.panelHeight() - 2 - 2 - nowPlayingInfoRows
	if rows > maxRows {
		rows = maxRows
	}
	if rows < 4 {
		rows = 4
	}
	return
}

// nowPlayingThumbCmd draws the currently-playing video's thumbnail into
// the Now Playing tab. No-op unless that tab is actually active and
// something's playing.
func (m Model) nowPlayingThumbCmd() tea.Cmd {
	if m.activeTab != tabNowPlaying || m.nowPlaying == nil || m.playingVid == nil || m.playingVid.Thumbnail == "" {
		return nil
	}
	if !thumb.Supported() || m.playingVid.ID == m.previewedID {
		return nil
	}
	v := *m.playingVid
	col, row, cols, rows := m.nowPlayingImageRegion()
	return func() tea.Msg {
		path, err := thumb.Download(v.Thumbnail, v.ID)
		if err != nil {
			return thumbErrMsg{err}
		}
		_ = thumb.Clear()
		if err := thumb.Show(path, col, row, cols, rows); err != nil {
			return thumbErrMsg{err}
		}
		return thumbShownMsg{id: v.ID}
	}
}

func (m Model) previewImageRegion() (col, row, cols, rows int) {
	listWidth, previewWidth := m.paneWidths()
	col = listWidth + 3                       // gap + preview panel's left border+padding
	row = headerLines + 1 + previewHeaderRows // header + panel top border + label rows

	cols = previewWidth - 4 // panel border(2) + padding(2)
	if cols < 10 {
		cols = 10
	}

	// YouTube thumbnails are 16:9. A terminal cell is roughly twice as
	// tall as it is wide, so the row count that makes the image actually
	// fill its box (instead of --scale-up preserving aspect and leaving a
	// gap before the text below it) is roughly cols * 9/16 * (cellW/cellH)
	// ≈ cols * 9/32. Without this, the reserved box was taller than the
	// image actually rendered, and the info text ended up looking like it
	// was glued to the bottom of the panel instead of the bottom of the
	// thumbnail.
	rows = int(math.Round(float64(cols) * 9.0 / 32.0))

	maxRows := m.panelHeight() - 2 - previewHeaderRows - previewFooterRows
	if rows > maxRows {
		rows = maxRows
	}
	if rows < 4 {
		rows = 4
	}
	return
}

// panelHeight is how tall the list/preview panels are — fills essentially
// all available vertical space between the header and footer, matching
// the "everything visible, fixed position" multi-panel convention.
func (m Model) panelHeight() int {
	h := m.height - headerLines - m.footerLines() - 1
	if h < 5 {
		h = 5
	}
	return h
}

// previewVisible reports whether the preview panel should actually be
// shown right now — the user's toggle is on, there's something to
// preview, AND the terminal is wide enough that showing it won't force
// the list panel below its own minimum width (which used to silently
// overflow the terminal on narrower/half-width windows instead of just
// hiding the panel).
func (m Model) previewVisible() bool {
	if !m.previewEnabled || len(m.list.Items()) == 0 {
		return false
	}
	const minListWidth = 20
	const widthGaps = 5 // JoinHorizontal separator + both panels' borders/padding slack
	if m.width < minPreviewPanelWidth+minListWidth+widthGaps {
		return false
	}
	const minImageRows = 4
	if m.panelHeight() < previewHeaderRows+previewFooterRows+minImageRows {
		return false
	}
	return true
}

// paneWidths returns (listWidth, previewWidth) in terminal columns. The
// preview panel has a fixed width (like a lazygit side panel); the list
// panel takes all remaining width, so it fills the screen on wide
// terminals instead of leaving dead space.
func (m Model) paneWidths() (listWidth, previewWidth int) {
	if !m.previewVisible() {
		return m.width - 2, 0
	}
	previewWidth = int(float64(m.width) * previewWidthFraction)
	if previewWidth > maxPreviewPanelWidth {
		previewWidth = maxPreviewPanelWidth
	}
	if previewWidth < minPreviewPanelWidth {
		previewWidth = minPreviewPanelWidth
	}
	listWidth = m.width - previewWidth - 3
	if listWidth < 20 {
		listWidth = 20
	}
	return
}

