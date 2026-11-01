package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/auth"
)

func (m Model) resolveLogoutConfirmation(key string, logout func(string) error) Model {
	if !m.logoutPending {
		return m
	}
	m.logoutPending = false
	if key != "y" {
		m.statusMsg = "Google logout dismissed"
		return m
	}
	if logout == nil {
		m.errMsg = "logout: authentication service unavailable"
		return m
	}
	if err := logout(m.activeProfile); err != nil {
		m.errMsg = "logout: " + err.Error()
		return m
	}
	m.loggedIn = false
	m.authClient = nil
	m.realSubscriptions = nil
	m.playlists = nil
	m.playlistVideos = nil
	m.browsingPlaylistID = ""
	m.channelVideos = nil
	m.browsingChannelID = ""
	m.likedVideos = nil
	m.homeFeed = nil
	m.errMsg = ""
	m.statusMsg = "logged out"
	return m
}

func loginCmd(profile string) tea.Cmd {
	return func() tea.Msg {
		err := auth.LoginForProfile(context.Background(), profile)
		return loginResultMsg{err: err}
	}
}

func importClientSecretsRawCmd(jsonText string) tea.Cmd {
	return func() tea.Msg {
		err := auth.ImportClientSecretsRaw(jsonText)
		return clientSecretsImportedMsg{err: err}
	}
}

func importClientCredentialsCmd(clientID, clientSecret string) tea.Cmd {
	return func() tea.Msg {
		err := auth.ImportClientCredentials(clientID, clientSecret)
		return clientSecretsImportedMsg{err: err}
	}
}

func authClientCmd(profile string) tea.Cmd {
	return func() tea.Msg {
		client, err := auth.ClientForProfile(context.Background(), profile)
		return authClientReadyMsg{client: client, err: err}
	}
}

