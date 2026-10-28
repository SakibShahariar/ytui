package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/thumb"
)

func audioOnlyLabel(enabled bool) string {
	if enabled {
		return "AUDIO"
	}
	return ""
}

func (m Model) View() string {
	var b strings.Builder

	loginStatus := lipgloss.NewStyle().Foreground(ctpOverlay0).Render("○ not logged in")
	if m.loggedIn {
		loginStatus = lipgloss.NewStyle().Foreground(ctpGreen).Render("●") + " " +
			lipgloss.NewStyle().Foreground(ctpSubtext0).Render("logged in")
	}
	titleLine := titleStyle.Render("ytui") + "  " + subtitleStyle.Render("youtube, in your terminal") +
		"   " + loginStatus
	b.WriteString(lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(titleLine))
	b.WriteString("\n")

	var tabsLine strings.Builder
	for _, t := range m.visibleTabs() {
		if t == m.activeTab {
			tabsLine.WriteString(activeTabStyle.Render(tabNames[t]))
		} else {
			tabsLine.WriteString(inactiveTabStyle.Render(tabNames[t]))
		}
	}
	b.WriteString(tabsLine.String())
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// --- Main panel row: list panel (+ optional preview panel) ---
	listWidth, previewWidth := m.paneWidths()
	panelH := m.panelHeight()

	var listContent strings.Builder
	if m.activeTab == tabSearch {
		listContent.WriteString(m.searchInput.View())
		if m.loading {
			listContent.WriteString("  " + m.spin.View())
		}
		listContent.WriteString("\n")
		listContent.WriteString(dividerStyle.Render(strings.Repeat("─", listWidth-2)))
		listContent.WriteString("\n")
	} else if m.libraryFilterMode {
		listContent.WriteString(m.libraryFilterInput.View())
		listContent.WriteString("\n")
		listContent.WriteString(dividerStyle.Render(strings.Repeat("─", listWidth-2)))
		listContent.WriteString("\n")
	} else if (m.activeTab == tabHistory || m.activeTab == tabDownloads) && m.libraryFilter != "" {
		listContent.WriteString(lipgloss.NewStyle().Foreground(ctpSubtext0).Render("Filter: "+m.libraryFilter))
		listContent.WriteString("\n")
		listContent.WriteString(dividerStyle.Render(strings.Repeat("─", listWidth-2)))
		listContent.WriteString("\n")
	}
	switch {
	case m.downloadPicker:
		listContent.WriteString(m.downloadPickerContent(listWidth - 2))
	case m.resumePrompt != nil:
		listContent.WriteString(m.resumePromptContent(listWidth - 2))
	case m.keyboardHelp:
		listContent.WriteString(m.keyboardHelpContent(listWidth-2, panelH-4))
	case m.showLoginHelp:
		listContent.WriteString(m.loginHelpContent(listWidth - 2))
	case m.activeTab == tabNowPlaying:
		listContent.WriteString(m.nowPlayingContent(listWidth - 2))
	case m.activeTab == tabSearch && !m.everSearched && len(m.list.Items()) == 0:
		listContent.WriteString(m.welcomeContent(listWidth-2, panelH-4))
	case !m.loggedIn && (m.activeTab == tabHome || m.activeTab == tabPlaylists || m.activeTab == tabLiked) && len(m.list.Items()) == 0:
		listContent.WriteString(m.notLoggedInContent(listWidth - 2))
	default:
		listContent.WriteString(m.list.View())
	}

	listPanel := panelBorder.Width(listWidth - 2).Height(panelH - 2).Render(listContent.String())

	if m.previewVisible() {
		var pv strings.Builder
		pv.WriteString(previewLabel.Render("Preview"))
		pv.WriteString("\n\n")

		_, _, _, imageRows := m.previewImageRegion()
		haveImage := thumb.Supported() && m.previewedID != ""
		if haveImage {
			pv.WriteString(strings.Repeat("\n", imageRows))
		} else {
			msg := "select a video to\npreview it here"
			if !thumb.Supported() {
				msg = "no preview\nKitty terminal required"
			}
			pv.WriteString(previewMutedText.Width(previewWidth - 4).Height(imageRows).Render(msg))
		}
		pv.WriteString("\n")

		innerW := previewWidth - 4
		if innerW < 8 {
			innerW = 8
		}
		footer := make([]string, previewFooterRows-1)
		if v, ok := m.selectedVideo(); ok {
			titleStyle := lipgloss.NewStyle().Foreground(ctpText).Bold(true)
			titleLine1, titleLine2 := wrapTwoLines(v.Title, innerW)
			footer[0] = titleStyle.MaxWidth(innerW).Render(titleLine1)
			footer[1] = titleStyle.MaxWidth(innerW).Render(titleLine2)
			footer[2] = dividerStyle.MaxWidth(innerW).Render(strings.Repeat("─", innerW))
			footer[3] = lipgloss.NewStyle().Foreground(ctpSubtext1).MaxWidth(innerW).
				Render("📺 " + truncate(v.Channel, innerW-4))
			if v.Duration > 0 {
				footer[4] = lipgloss.NewStyle().Foreground(ctpSubtext0).MaxWidth(innerW).
					Render("⏱  " + formatDuration(v.Duration))
			}
			if v.ViewCount > 0 {
				footer[5] = lipgloss.NewStyle().Foreground(ctpSubtext0).MaxWidth(innerW).
					Render("👁  " + truncate(formatViews(v.ViewCount)+" views", innerW-4))
			}
		}
		pv.WriteString(strings.Join(footer, "\n"))

		previewPanel := panelBorder.Width(previewWidth - 2).Height(panelH - 2).Render(pv.String())
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", previewPanel))
	} else {
		b.WriteString(listPanel)
	}
	b.WriteString("\n")

	// --- Footer: solid status bar, like a statusline, not floating text ---
	modeBar := footerModeNav.Render("BROWSE")
	help := m.contextualHelp()
	if m.downloadPicker {
		modeBar = footerModeInput.Render("DOWNLOAD")
		help = "↑↓: move · enter/space: select · esc: cancel"
	} else if m.libraryFilterMode {
		modeBar = footerModeInput.Render("FILTER")
		help = "enter: apply · ctrl+u: clear text · esc: cancel"
	} else if m.resumePrompt != nil {
		modeBar = footerModeInput.Render("RESUME")
		help = "enter/r: resume · b: start from beginning · esc: cancel"
	} else if m.keyboardHelp {
		modeBar = footerModeInput.Render("HELP")
		help = "↑↓: scroll · pgup/pgdn: page · home/end: jump · esc or ?: close"
	} else if m.inputMode {
		modeBar = footerModeInput.Render("SEARCH")
		help = "enter: run search · ↑↓: back to browsing · esc: cancel · ctrl+c: quit"
	}
	if !m.libraryFilterMode && (m.activeTab == tabHistory || m.activeTab == tabDownloads) {
		help += " · ctrl+f: filter · ctrl+y: date " + m.libraryDateFilterLabel() + " · ctrl+u: clear"
		if m.activeTab == tabDownloads && !m.offlineDownloads {
			help += " · ctrl+g: status " + m.downloadStatusFilterLabel()
		}
	}
	if label := audioOnlyLabel(m.audioOnly); label != "" {
		modeBar = lipgloss.JoinHorizontal(lipgloss.Top, modeBar, " ", footerModeInput.Render(label))
	}
	loadingIndicator := ""
	if m.ytLoading {
		loadingIndicator = "  " + m.spin.View()
	}
	footerText := " " + help + loadingIndicator
	avail := m.width - lipgloss.Width(modeBar)
	footerLine := lipgloss.JoinHorizontal(lipgloss.Top, modeBar,
		footerBarStyle.Width(avail).MaxWidth(avail).Render(footerText))
	b.WriteString(footerLine)

	if m.errMsg != "" {
		b.WriteString("\n" + errorStyle.Render("error: "+m.errMsg))
	} else if m.nowPlaying != nil {
		b.WriteString("\n" + m.nowPlayingBar())
	} else if m.statusMsg != "" {
		b.WriteString("\n" + dividerStyle.Render(m.statusMsg))
	}

	return b.String()
}

// nowPlayingBar renders the 2-line "now playing" display: title + mode +
// volume on the first line, a progress bar with elapsed/total time on the
// second. Replaces what used to be a single line of plain status text.
func (m Model) nowPlayingBar() string {
	playIcon := "▶"
	if m.npPaused {
		playIcon = "⏸"
	}
	mode := "video"
	if m.audioOnly {
		mode = "audio"
	}
	title := ""
	if m.playingVid != nil {
		title = m.playingVid.Title
	}
	volPct := int(m.npVol)
	titleLine := lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).Render(playIcon+" ") +
		lipgloss.NewStyle().Foreground(ctpText).Render(truncate(title, m.width-24)) +
		lipgloss.NewStyle().Foreground(ctpOverlay1).Render(fmt.Sprintf("  (%s)", mode))
	volLine := lipgloss.NewStyle().Foreground(ctpSubtext0).Render(fmt.Sprintf("🔊 %d%%", volPct))
	titleRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width-lipgloss.Width(volLine)).Render(titleLine),
		volLine,
	)

	bar := progressBar(percentOf(m.npPos, m.npDur), 30)
	timeStr := fmt.Sprintf("%s / %s", formatDuration(m.npPos), formatDuration(m.npDur))
	progressRow := lipgloss.NewStyle().Foreground(ctpLavender).Render(bar) + "  " +
		lipgloss.NewStyle().Foreground(ctpSubtext0).Render(timeStr)

	return titleRow + "\n" + progressRow
}

// percentOf is a small guard against divide-by-zero before mpv has
// reported a duration yet (right after playback starts).
func percentOf(pos, dur float64) float64 {
	if dur <= 0 {
		return 0
	}
	return pos / dur * 100
}
