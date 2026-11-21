package store

import (
	"path/filepath"
	"testing"
)

func TestChannelPrefsPersist(t *testing.T) {
	t.Setenv("YTUI_DATA_PATH", filepath.Join(t.TempDir(), "data.json"))
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	pref := ChannelPref{Group: "Anime", Favorite: true, Muted: true}
	if err := s.SetChannelPref("UC123", pref); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.ChannelPref("UC123")
	if got != pref {
		t.Fatalf("persisted pref = %#v, want %#v", got, pref)
	}
	if err := reloaded.SetChannelPref("UC123", ChannelPref{}); err != nil {
		t.Fatal(err)
	}
	if reloaded.ChannelPref("UC123") != (ChannelPref{}) {
		t.Fatal("clearing a preference did not remove it")
	}
}
