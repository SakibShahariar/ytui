package store

import "strings"

// ChannelPref holds local-only organization for a subscribed channel. It is
// never synced to YouTube and never affects subscriptions on the service.
type ChannelPref struct {
	Group    string `json:"group,omitempty"`
	Favorite bool   `json:"favorite,omitempty"`
	Muted    bool   `json:"muted,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
}

// ChannelPref returns the stored preferences for a channel (zero value if
// none have been set).
func (s *Store) ChannelPref(channelID string) ChannelPref {
	if s.ChannelPrefs == nil {
		return ChannelPref{}
	}
	return s.ChannelPrefs[channelID]
}

// SetChannelPref stores preferences for a channel and persists them.
func (s *Store) SetChannelPref(channelID string, pref ChannelPref) error {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil
	}
	pref.Group = strings.TrimSpace(pref.Group)
	if s.ChannelPrefs == nil {
		s.ChannelPrefs = make(map[string]ChannelPref)
	}
	if pref == (ChannelPref{}) {
		delete(s.ChannelPrefs, channelID)
	} else {
		s.ChannelPrefs[channelID] = pref
	}
	return s.Save()
}
