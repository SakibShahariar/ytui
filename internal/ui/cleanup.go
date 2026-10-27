package ui

import (
	"errors"

	"github.com/SakibShahariar/ytui/internal/thumb"
)

// cleanupResources centralizes shutdown ordering and is injectable in tests,
// so cleanup can be verified without launching mpv or a Kitty terminal.
func cleanupResources(stopPlayer func() error, stopDownloads func(), clearThumbnail func() error) error {
	var errs []error
	if stopPlayer != nil {
		if err := stopPlayer(); err != nil {
			errs = append(errs, err)
		}
	}
	if stopDownloads != nil {
		stopDownloads()
	}
	if clearThumbnail != nil {
		if err := clearThumbnail(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Cleanup releases resources owned by the UI model. It is safe to call when
// a resource was never initialized and is intended to run after every TUI exit.
func (m Model) Cleanup() error {
	var stopPlayer func() error
	if m.nowPlaying != nil {
		stopPlayer = func() error {
			m.saveResumePosition()
			return m.nowPlaying.Quit()
		}
	}
	var stopDownloads func()
	if m.downloads != nil {
		stopDownloads = m.downloads.Shutdown
	}
	var clearThumbnail func() error
	if m.previewedID != "" && thumb.Supported() {
		clearThumbnail = thumb.Clear
	}
	return cleanupResources(stopPlayer, stopDownloads, clearThumbnail)
}
