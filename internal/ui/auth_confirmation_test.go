package ui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestCtrlLRequestsGoogleLogoutConfirmation(t *testing.T) {
	m := Model{
		loggedIn: true,
		activeProfile: "personal",
		homeFeed: []list.Item{videoItem{v: ytdlp.Video{ID: "cached"}}},
	}
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlL})
	got := updated.(Model)
	if cmd != nil { t.Fatalf("Ctrl+L command = %v, want nil", cmd) }
	if !got.logoutPending { t.Fatal("Ctrl+L did not request logout confirmation") }
	if !got.loggedIn || len(got.homeFeed) != 1 { t.Fatal("Ctrl+L cleared authenticated state before confirmation") }
	if got.statusMsg == "" { t.Fatal("logout prompt did not identify the active profile") }
}

func TestResolveLogoutConfirmationDismissesWithoutLoggingOut(t *testing.T) {
	called := false
	m := Model{loggedIn: true, activeProfile: "personal", logoutPending: true}
	got := m.resolveLogoutConfirmation("n", func(string) error { called = true; return nil })
	if called { t.Fatal("dismissed logout invoked the logout operation") }
	if !got.loggedIn { t.Fatal("dismissed logout cleared login state") }
	if got.logoutPending { t.Fatal("dismissal left logout pending") }
	if got.statusMsg != "Google logout dismissed" { t.Fatalf("status = %q", got.statusMsg) }
}

func TestResolveLogoutConfirmationClearsStateOnlyAfterSuccess(t *testing.T) {
	m := Model{
		loggedIn: true,
		activeProfile: "personal",
		logoutPending: true,
		homeFeed: []list.Item{videoItem{v: ytdlp.Video{ID: "cached"}}},
		playlists: []list.Item{playlistItemUI{}},
		realSubscriptions: []list.Item{realSubItem{}},
	}
	loggedOutProfile := ""
	got := m.resolveLogoutConfirmation("y", func(profile string) error {
		loggedOutProfile = profile
		return nil
	})
	if loggedOutProfile != "personal" { t.Fatalf("logged out profile = %q, want personal", loggedOutProfile) }
	if got.loggedIn || got.logoutPending { t.Fatal("confirmed logout did not clear login/pending state") }
	if got.homeFeed != nil || got.playlists != nil || got.realSubscriptions != nil {
		t.Fatal("confirmed logout did not clear account-scoped in-memory data")
	}
	if got.statusMsg != "logged out" { t.Fatalf("status = %q", got.statusMsg) }
}

func TestResolveLogoutConfirmationPreservesStateWhenLogoutFails(t *testing.T) {
	m := Model{loggedIn: true, activeProfile: "personal", logoutPending: true}
	wantErr := errors.New("token removal failed")
	got := m.resolveLogoutConfirmation("y", func(string) error { return wantErr })
	if !got.loggedIn { t.Fatal("failed logout cleared login state") }
	if got.logoutPending { t.Fatal("failed logout left confirmation pending") }
	if got.errMsg != "logout: token removal failed" { t.Fatalf("error = %q", got.errMsg) }
}
