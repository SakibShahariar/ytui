package ui

import (
	"strings"

	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/thumb"
)

func (m Model) contextualHelp() string {
	parts := []string{"tab: switch view"}
	if m.activeTab == tabSearch {
		parts = append(parts, "type to search")
	}

	hasItems := len(m.list.Items()) > 0
	if hasItems {
		parts = append(parts, "↑↓: move")
	}

	if hasItems {
		switch m.list.SelectedItem().(type) {
		case playlistItemUI:
			parts = append(parts, "enter: open playlist")
		case realSubItem:
			parts = append(parts, "enter: open channel")
		default:
			if _, ok := m.selectedVideo(); ok {
				parts = append(parts, "enter: play")
			}
		}
	}

	if _, ok := m.selectedVideo(); ok {
		parts = append(parts, "^s: sub", "^w: later", "^d: download")
	}
	if m.activeTab == tabDownloads {
		if m.offlineDownloads {
			if hasItems { parts = append(parts, "enter: play", "delete: remove (confirm with y)") }
			parts = append(parts, "^o: queue")
		} else {
			parts = append(parts, "^o: offline files")
		}
	}
	if hasItems && m.activeTab == tabDownloads && !m.offlineDownloads {
		if item, ok := m.list.SelectedItem().(downloadItem); ok {
			if item.d.Status == downloader.StatusQueued || item.d.Status == downloader.StatusDownloading {
				parts = append(parts, "^d: cancel")
			}
			if item.d.Status == downloader.StatusError || item.d.Status == downloader.StatusResumable || item.d.Status == downloader.StatusCanceled {
				parts = append(parts, "^r: retry/resume")
			}
		}
	}

	parts = append(parts, "^a: audio")

	if hasItems && thumb.Supported() {
		parts = append(parts, "^t: thumbs")
	}

	if !m.loggedIn {
		parts = append(parts, "^l: login")
	}
	parts = append(parts, "^c: quit")

	return strings.Join(parts, " · ")
}

