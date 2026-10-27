package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/library"
)

func (m *Model) applyListSize() {
	listWidth, _ := m.paneWidths()
	listHeight := m.panelHeight() - 2 // panel top+bottom border
	if m.activeTab == tabSearch {
		listHeight -= 2 // search input line + divider line rendered above the list
	}
	m.list.SetSize(listWidth-2, listHeight)
}

// moveSelection forwards a navigation key to the list widget and, if the
// selection actually changed, kicks off a thumbnail redraw.
func (m Model) moveSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prevIndex := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if m.list.Index() != prevIndex {
		if v, ok := m.selectedVideo(); ok {
			if thumbCmd := m.showThumbnailCmd(v); thumbCmd != nil {
				return m, tea.Batch(cmd, thumbCmd)
			}
		}
	}
	return m, cmd
}

func (m *Model) refreshList() {
	if m.activeTab == tabDownloads && m.offlineDownloads {
		files, err := library.Scan(m.downloadDirectory())
		if err != nil {
			m.offlineFiles = nil
			m.errMsg = "offline downloads: " + err.Error()
		} else {
			m.offlineFiles = files
			m.errMsg = ""
		}
	}
	m.list.SetItems(m.currentListItems())
	switch {
	case m.activeTab == tabDownloads && m.offlineDownloads:
		m.list.Title = "Offline Downloads"
	case m.activeTab == tabPlaylists && m.browsingPlaylistID != "":
		m.list.Title = m.browsingPlaylistTitle
	case m.activeTab == tabSubscriptions && m.browsingChannelID != "":
		m.list.Title = m.browsingChannelTitle
	default:
		m.list.Title = tabNames[m.activeTab]
	}
	m.applyListSize()
}

// contextualHelp builds the BROWSE-mode footer hint, showing only
// keybinds that would actually do something right now — e.g. ^s (subscribe)
// only appears when a video is actually selected, not on every screen
// regardless of whether there's anything to subscribe to.
