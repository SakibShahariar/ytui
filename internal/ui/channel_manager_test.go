package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/store"
)

func channelTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddChannel(store.Channel{ID: "UCa", Name: "Alpha", URL: "https://example.com/a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddChannel(store.Channel{ID: "UCb", Name: "Beta", URL: "https://example.com/b"}); err != nil {
		t.Fatal(err)
	}
	m := Model{store: s, activeTab: tabSubscriptions, channelGroupInput: textinput.New()}
	m.list = list.New(nil, ytDelegate{}, 60, 10)
	return m
}

func TestChannelManagerTogglesPrefs(t *testing.T) {
	m := channelTestModel(t)
	m.channelManager = true
	updated, _ := m.handleChannelManagerKey("f", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if !m.store.ChannelPref("UCa").Favorite {
		t.Fatal("favorite toggle did not persist")
	}
	updated, _ = m.handleChannelManagerKey("h", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)
	if !m.store.ChannelPref("UCa").Hidden {
		t.Fatal("hide toggle did not persist")
	}
	view := m.channelManagerContent(60)
	if !strings.Contains(view, "★ favorite") || !strings.Contains(view, "hidden") {
		t.Fatalf("manager view missing markers:\n%s", view)
	}
}

func TestChannelManagerGroupEdit(t *testing.T) {
	m := channelTestModel(t)
	m.channelManager = true
	updated, _ := m.handleChannelManagerKey("g", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if !m.channelGroupEditing {
		t.Fatal("g did not open group editor")
	}
	m.channelGroupInput.SetValue("Anime")
	updated, _ = m.handleChannelManagerKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.channelGroupInput.Value() == "" || m.channelGroupEditing || m.store.ChannelPref("UCa").Group != "Anime" {
		t.Fatalf("group edit failed: editing=%v, pref=%#v", m.channelGroupEditing, m.store.ChannelPref("UCa"))
	}
}

func TestFilterChannelsHidesAndFavoritesFirst(t *testing.T) {
	m := channelTestModel(t)
	items := []list.Item{
		subItem{c: store.Channel{ID: "UCa", Name: "Alpha"}},
		subItem{c: store.Channel{ID: "UCb", Name: "Beta"}},
	}
	if err := m.store.SetChannelPref("UCa", store.ChannelPref{Favorite: true}); err != nil {
		t.Fatal(err)
	}
	filtered := m.filterChannels(items, channelIDForSubscriptions)
	if len(filtered) != 2 || filtered[0].(subItem).Title() != "Alpha" {
		t.Fatalf("favorite ordering = %v", filtered)
	}
	if err := m.store.SetChannelPref("UCb", store.ChannelPref{Hidden: true}); err != nil {
		t.Fatal(err)
	}
	filtered = m.filterChannels(items, channelIDForSubscriptions)
	if len(filtered) != 1 || filtered[0].(subItem).Title() != "Alpha" {
		t.Fatalf("hidden filtering = %v", filtered)
	}
}
