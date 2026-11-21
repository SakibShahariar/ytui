package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/store"
)

type managedChannel struct {
	id   string
	name string
}

// managedChannels combines local and API subscriptions into one deduplicated
// list for the local channel manager.
func (m Model) managedChannels() []managedChannel {
	seen := make(map[string]bool)
	channels := make([]managedChannel, 0)
	add := func(id, name string) {
		if id == "" || seen[id] || name == "" {
			return
		}
		seen[id] = true
		channels = append(channels, managedChannel{id: id, name: name})
	}
	for _, c := range m.store.Subscriptions {
		add(c.ID, c.Name)
	}
	for _, item := range m.realSubscriptions {
		if sub, ok := item.(realSubItem); ok {
			add(sub.s.ChannelID, sub.s.Title)
		}
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].name < channels[j].name })
	return channels
}

// filterChannels applies local preferences: hidden channels disappear and
// favorites are listed first. It never contacts YouTube.
func (m Model) filterChannels(items []list.Item, idFor func(list.Item) string) []list.Item {
	filtered := make([]list.Item, 0, len(items))
	for _, item := range items {
		if idFor(item) != "" && m.store.ChannelPref(idFor(item)).Hidden {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		iFav := idFor(filtered[i]) != "" && m.store.ChannelPref(idFor(filtered[i])).Favorite
		jFav := idFor(filtered[j]) != "" && m.store.ChannelPref(idFor(filtered[j])).Favorite
		return iFav && !jFav
	})
	return filtered
}

func channelPrefMarkers(pref store.ChannelPref) string {
	markers := make([]string, 0, 3)
	if pref.Favorite {
		markers = append(markers, "★ favorite")
	}
	if pref.Muted {
		markers = append(markers, "muted")
	}
	if pref.Hidden {
		markers = append(markers, "hidden")
	}
	if pref.Group != "" {
		markers = append(markers, "group: "+pref.Group)
	}
	return strings.Join(markers, " · ")
}

func (m Model) channelManagerContent(width int) string {
	if width < 20 {
		width = 20
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("Manage channels (local preferences)"),
		lipgloss.NewStyle().Foreground(ctpOverlay0).Render("These settings never change your YouTube subscriptions."),
		"",
	}
	channels := m.managedChannels()
	if len(channels) == 0 {
		lines = append(lines, "No subscriptions to manage yet.")
	}
	for i, channel := range channels {
		marker := "  "
		if i == m.channelCursor {
			marker = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).Render("▸ ")
		}
		pref := m.store.ChannelPref(channel.id)
		line := fmt.Sprintf("%-28s", truncate(channel.name, 28))
		if markers := channelPrefMarkers(pref); markers != "" {
			line += "  " + markers
		}
		if i == m.channelCursor && m.channelGroupEditing {
			line += "  group: " + m.channelGroupInput.View()
		}
		lines = append(lines, marker+line)
	}
	lines = append(lines, "", "↑↓: select · f: favorite · m: mute · h: hide · g: set group · esc: close")
	return strings.Join(lines, "\n")
}

func (m Model) openChannelManager() (tea.Model, tea.Cmd) {
	m.channelManager = true
	m.channelCursor = 0
	m.channelGroupEditing = false
	m.channelGroupInput = textinput.New()
	return m, nil
}

func (m Model) toggleChannelPref(field string) (tea.Model, tea.Cmd) {
	channels := m.managedChannels()
	if m.channelCursor >= len(channels) {
		m.errMsg = "no channel selected"
		return m, nil
	}
	channel := channels[m.channelCursor]
	pref := m.store.ChannelPref(channel.id)
	switch field {
	case "favorite":
		pref.Favorite = !pref.Favorite
	case "muted":
		pref.Muted = !pref.Muted
	case "hidden":
		pref.Hidden = !pref.Hidden
	}
	if err := m.store.SetChannelPref(channel.id, pref); err != nil {
		m.errMsg = "save channel preference: " + err.Error()
	} else {
		m.errMsg = ""
		m.statusMsg = channel.name + ": " + field + " updated"
	}
	m.refreshList()
	return m, nil
}

func (m Model) handleChannelManagerKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	channels := m.managedChannels()
	if m.channelGroupEditing {
		switch key {
		case "enter":
			if m.channelCursor < len(channels) {
				channel := channels[m.channelCursor]
				pref := m.store.ChannelPref(channel.id)
				pref.Group = strings.TrimSpace(m.channelGroupInput.Value())
				if err := m.store.SetChannelPref(channel.id, pref); err != nil {
					m.errMsg = "save group: " + err.Error()
				} else {
					m.errMsg = ""
					m.statusMsg = channel.name + ": group updated"
				}
				m.refreshList()
			}
			m.channelGroupEditing = false
			m.channelGroupInput.Blur()
			return m, nil
		case "esc":
			m.channelGroupEditing = false
			m.channelGroupInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.channelGroupInput, cmd = m.channelGroupInput.Update(msg)
			return m, cmd
		}
	}
	switch key {
	case "esc", "q":
		m.channelManager = false
		return m, nil
	case "up":
		if m.channelCursor > 0 {
			m.channelCursor--
		}
		return m, nil
	case "down":
		if m.channelCursor < len(channels)-1 {
			m.channelCursor++
		}
		return m, nil
	case "f":
		return m.toggleChannelPref("favorite")
	case "m":
		return m.toggleChannelPref("muted")
	case "h":
		return m.toggleChannelPref("hidden")
	case "g":
		if m.channelCursor < len(channels) {
			m.channelGroupEditing = true
			m.channelGroupInput.SetValue(m.store.ChannelPref(channels[m.channelCursor].id).Group)
			m.channelGroupInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

// channelIDForSubscriptions extracts channel IDs for list items.
func channelIDForSubscriptions(item list.Item) string {
	switch it := item.(type) {
	case subItem:
		return it.c.ID
	case realSubItem:
		return it.s.ChannelID
	}
	return ""
}

// channelPrefForID is used by list item descriptions.
func channelPrefForID(m Model, id string) store.ChannelPref { return m.store.ChannelPref(id) }
