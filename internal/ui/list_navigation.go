package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/library"
)

func (m *Model) applyListSize() {
	listWidth, _ := m.paneWidths()
	listHeight := m.panelHeight() - 2 // panel top+bottom border
	if m.activeTab == tabSearch || ((m.activeTab == tabHistory || m.activeTab == tabDownloads) && (m.libraryFilterMode || m.libraryFilter != "")) {
		listHeight -= 2 // input/filter line + divider line rendered above the list
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

func clampListIndex(index, itemCount int) int {
	if itemCount <= 0 || index < 0 {
		return 0
	}
	if index >= itemCount {
		return itemCount - 1
	}
	return index
}

func (m *Model) restoreListPosition() {
	if m.listPositions == nil {
		m.listPositions = make(map[tab]int)
	}
	index := clampListIndex(m.listPositions[m.activeTab], len(m.list.Items()))
	m.list.Select(index)
	m.listPositions[m.activeTab] = index
}

func (m *Model) refreshList() {
	// The shared list widget still contains items from listTab, even if a
	// caller already changed activeTab. Save against the tab those items
	// belong to before replacing them.
	if m.listTabReady {
		if m.listPositions == nil {
			m.listPositions = make(map[tab]int)
		}
		m.listPositions[m.listTab] = m.list.Index()
	}
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
	items := m.currentListItems()
	if m.activeTab == tabHistory || m.activeTab == tabDownloads {
		items = m.filterLibraryItems(items)
	}
	m.list.SetItems(items)
	m.restoreListPosition()
	m.listTab = m.activeTab
	m.listTabReady = true
	switch {
	case m.activeTab == tabDownloads && m.offlineDownloads:
		m.list.Title = "Offline Downloads"
	case m.activeTab == tabPlaylists && m.browsingPlaylistID != "":
		m.list.Title = m.browsingPlaylistTitle
	case m.activeTab == tabSubscriptions && m.browsingChannelID != "":
		m.list.Title = m.browsingChannelTitle
	case m.activeTab == tabSearch && m.searchPlaylistTitle != "":
		m.list.Title = m.searchPlaylistTitle
	default:
		m.list.Title = tabNames[m.activeTab]
	}
	m.applyListSize()
}

// contextualHelp builds the BROWSE-mode footer hint, showing only
// keybinds that would actually do something right now — e.g. ^s (subscribe)
// only appears when a video is actually selected, not on every screen
// regardless of whether there's anything to subscribe to.
