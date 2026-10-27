package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/thumb"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

// refreshPreview clears any currently-drawn thumbnail and redraws one for
// whatever's now selected, if applicable. Call this any time the *visible
// content* changes wholesale (switching tabs, drilling into/out of a
// playlist) — not just when moving the cursor within the same list.
// Without this, an old thumbnail (drawn outside Bubbletea's own render
// loop, so it isn't naturally cleared by a normal redraw) stays on screen
// after switching to a tab/state that has nothing to do with it.
func (m *Model) refreshPreview() tea.Cmd {
	_ = thumb.Clear()
	m.previewedID = ""
	if m.activeTab == tabNowPlaying {
		return m.nowPlayingThumbCmd()
	}
	if !m.previewVisible() {
		return nil
	}
	if v, ok := m.selectedVideo(); ok {
		return m.showThumbnailCmd(v)
	}
	return nil
}

// showThumbnailCmd downloads (if needed) and draws the thumbnail for v into
// the preview panel region. No-op if thumbnails aren't supported/enabled or
// the video has no thumbnail URL.
func (m Model) showThumbnailCmd(v ytdlp.Video) tea.Cmd {
	if !m.previewVisible() || v.Thumbnail == "" || v.ID == m.previewedID {
		return nil
	}
	col, row, cols, rows := m.previewImageRegion()
	return func() tea.Msg {
		path, err := thumb.DownloadWithPolicy(v.Thumbnail, v.ID, thumb.CachePolicy{
			MaxEntries: m.cfg.ThumbnailCacheMaxEntries,
			MaxAgeDays: m.cfg.ThumbnailCacheMaxAgeDays,
		})
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

// previewImageRegion computes the absolute terminal (col, row, cols, rows)
// the thumbnail image should draw into, inset a little from the preview
// panel's own borders.
// Preview panel content is a fixed vertical recipe: label, blank, image
// area, then a fixed-size info block. previewImageRegion and the panel
// renderer in View() must agree on these exactly, or the image ends up
// drawn over the label text instead of below it.
const (
	previewHeaderRows = 2 // "Preview" label + blank line
	previewFooterRows = 7 // blank separator, title (2 lines), divider, channel, duration, views
)

// nowPlayingInfoRows is how many lines the Now Playing tab's info block
// (below the thumbnail) reserves: blank, title (2 lines), divider,
// channel/duration/views, blank, description label, then description text
// itself gets whatever's left. Kept as a named constant — shared between
// the region math and the renderer — specifically because mismatched
// "how many lines does this reserve" constants have been the source of
// real layout bugs here before (image overlapping text, overflow).
const nowPlayingInfoRows = 7

