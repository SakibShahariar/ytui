package ui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/auth"
	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/thumb"
)

func New(s *store.Store) Model {
	ti := textinput.New()
	ti.Placeholder = "Search YouTube... (press enter)"
	ti.Prompt = "🔍 "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ctpMauve)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ctpMauve)
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	pi := textinput.New()
	pi.Placeholder = "paste here…"
	pi.PromptStyle = lipgloss.NewStyle().Foreground(ctpGreen)
	pi.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
	pi.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
	pi.Cursor.Style = lipgloss.NewStyle().Foreground(ctpGreen)
	pi.CharLimit = 4000
	pi.Width = 60

	delegate := ytDelegate{}

	l := list.New(nil, delegate, defaultWidth, defaultHeight-headerLines-2)
	l.Title = "Results"
	l.Styles.Title = lipgloss.NewStyle().Background(ctpMauve).Foreground(ctpBase).Bold(true).Padding(0, 1)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we render our own help line, keyed to mode
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	cfg, cfgErr := config.LoadWithError()
	if override := os.Getenv("YTUI_DOWNLOAD_DIR"); override != "" {
		cfg.DownloadDir = override
	}
	dl, downloadErr := downloader.New(cfg.DownloadDir)

	m := Model{
		store:          s,
		cfg:            cfg,
		activeTab:      tabSearch,
		inputMode:      true,
		searchInput:    ti,
		pathInput:      pi,
		list:           l,
		listPositions:  make(map[tab]int),
		listTab:        tabSearch,
		listTabReady:   true,
		spin:           sp,
		width:          defaultWidth,
		height:         defaultHeight,
		previewEnabled: thumb.Supported(), // auto-on in Kitty; press 't' to disable if it glitches
		downloads:      dl,
		audioOnly:      cfg.DefaultAudioOnly,
	}
	if cfgErr != nil {
		m.errMsg = "config: " + cfgErr.Error()
	}
	if downloadErr != nil {
		if m.errMsg != "" {
			m.errMsg += "; "
		}
		m.errMsg += "downloads: " + downloadErr.Error()
	}
	if dl != nil {
		if removed, cleanupErr := dl.StaleCleanupResult(); cleanupErr != nil {
			m.statusMsg = "partial-file cleanup warning: " + cleanupErr.Error()
		} else if removed > 0 {
			m.statusMsg = fmt.Sprintf("removed %d stale partial download file(s)", removed)
		}
	}
	if warning := dependencyWarning(exec.LookPath); warning != "" {
		if m.errMsg != "" {
			m.errMsg += "; " + warning
		} else if m.statusMsg != "" {
			m.statusMsg += "; " + warning
		} else {
			m.statusMsg = warning
		}
	}
	m.applyListSize()
	return m
}

func (m Model) Init() tea.Cmd {
	if auth.HaveToken() {
		return tea.Batch(textinput.Blink, authClientCmd())
	}
	return textinput.Blink
}

