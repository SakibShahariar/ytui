package ui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"ytui/internal/downloader"
)

func (m Model) downloadDirectory() string {
	if m.downloads != nil { return m.downloads.DestDir() }
	if m.cfg.DownloadDir != "" { return m.cfg.DownloadDir }
	home, err := os.UserHomeDir()
	if err != nil { return "" }
	return filepath.Join(home, "Downloads", "ytui")
}

