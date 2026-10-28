package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/auth"
)

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

