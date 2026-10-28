package ui

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/SakibShahariar/ytui/internal/config"
    "github.com/SakibShahariar/ytui/internal/sponsorblock"
)

var defaultSponsorBlockSkipCategories = []string{"sponsor", "selfpromo"}

func containsSponsorCategory(categories []string, wanted string) bool {
    for _, category := range categories { if category == wanted { return true } }
    return false
}

func (m Model) effectiveSponsorBlockCategories() []string {
    if m.cfg.SponsorBlockCategoriesConfigured {
        return append([]string(nil), m.cfg.SponsorBlockCategories...)
    }
    return append([]string(nil), defaultSponsorBlockSkipCategories...)
}

func (m Model) openSponsorBlockSettings() Model {
    m.sponsorBlockSettings = true
    m.sponsorBlockCursor = 0
    m.sponsorBlockDraftEnabled = m.cfg.SponsorBlockEnabled
    m.sponsorBlockDraftCategories = m.effectiveSponsorBlockCategories()
    m.inputMode = false
    m.searchInput.Blur()
    return m
}

func (m Model) toggleSponsorBlockSetting() Model {
    if m.sponsorBlockCursor == 0 {
        m.sponsorBlockDraftEnabled = !m.sponsorBlockDraftEnabled
        return m
    }
    index := m.sponsorBlockCursor - 1
    if index < 0 || index >= len(sponsorblock.SkipCategories) { return m }
    category := sponsorblock.SkipCategories[index]
    if containsSponsorCategory(m.sponsorBlockDraftCategories, category) {
        filtered := make([]string, 0, len(m.sponsorBlockDraftCategories)-1)
        for _, current := range m.sponsorBlockDraftCategories {
            if current != category { filtered = append(filtered, current) }
        }
        m.sponsorBlockDraftCategories = filtered
    } else {
        m.sponsorBlockDraftCategories = append(m.sponsorBlockDraftCategories, category)
    }
    return m
}

func (m Model) saveSponsorBlockSettings() (tea.Model, tea.Cmd) {
    m.cfg.SponsorBlockEnabled = m.sponsorBlockDraftEnabled
    m.cfg.SponsorBlockCategories = append([]string{}, m.sponsorBlockDraftCategories...)
    m.cfg.SponsorBlockCategoriesConfigured = true
    if err := config.Save(m.cfg); err != nil {
        m.errMsg = "save SponsorBlock settings: " + err.Error()
        return m, nil
    }
    m.sponsorBlockSettings = false
    m.errMsg = ""
    m.sponsorLastSkipped = ""
    if !m.cfg.SponsorBlockEnabled {
        m.sponsorSegments = nil
        m.sponsorSegmentVideoID = ""
        m.statusMsg = "SponsorBlock disabled"
        return m, nil
    }
    if m.playingVid != nil && !m.playingLocal {
        m.sponsorSegments = nil
        m.sponsorSegmentVideoID = ""
        m.statusMsg = "SponsorBlock enabled; fetching segments..."
        return m, loadSponsorSegmentsCmd(m.playingVid.ID)
    }
    m.statusMsg = "SponsorBlock enabled; segments load when a video starts"
    return m, nil
}

func (m Model) sponsorBlockSettingsContent(width int) string {
    if width < 12 { width = 12 }
    lines := []string{
        lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("SponsorBlock settings"),
        "Optional segment lookup uses a hashed video-ID prefix for privacy.",
        "",
    }
    rows := []string{"SponsorBlock integration"}
    rows = append(rows, sponsorblock.SkipCategories...)
    for i, label := range rows {
        checked := false
        if i == 0 {
            checked = m.sponsorBlockDraftEnabled
        } else {
            checked = containsSponsorCategory(m.sponsorBlockDraftCategories, label)
        }
        prefix := "[ ] "
        if checked { prefix = "[x] " }
        style := lipgloss.NewStyle().Foreground(ctpText)
        if i == m.sponsorBlockCursor {
            prefix = lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("> "+prefix)
            style = style.Bold(true).Foreground(ctpMauve)
        } else {
            prefix = "  " + prefix
        }
        value := prefix + label
        if i > 0 && !checked { value += " - highlight/manual skip only" }
        lines = append(lines, style.MaxWidth(width).Render(value))
    }
    lines = append(lines, "", lipgloss.NewStyle().Foreground(ctpOverlay1).Render("Up/Down: select; Space: toggle; Enter: save; Esc: cancel"))
    return strings.Join(lines, "\n")
}

func (m Model) skipCurrentSponsorSegment() (tea.Model, tea.Cmd) {
    if !m.cfg.SponsorBlockEnabled || m.nowPlaying == nil || m.playingVid == nil || m.playingLocal {
        m.statusMsg = "SponsorBlock is not active for this playback"
        return m, nil
    }
    for _, segment := range m.sponsorSegments {
        if segment.ActionType != "skip" || m.npPos < segment.Start || m.npPos >= segment.End { continue }
        if err := m.nowPlaying.SeekAbsolute(segment.End); err != nil {
            m.statusMsg = "could not skip segment: " + err.Error()
            return m, nil
        }
        m.npPos = segment.End
        m.sponsorLastSkipped = sponsorSegmentKey(segment)
        m.statusMsg = fmt.Sprintf("manually skipped %s segment (%s-%s)", segment.Category, formatDuration(segment.Start), formatDuration(segment.End))
        return m, nil
    }
    if highlight := m.nextSponsorHighlight(); highlight != "" {
        m.statusMsg = "no active segment; " + highlight
    } else {
        m.statusMsg = "no SponsorBlock segment at the current position"
    }
    return m, nil
}

func sponsorSegmentKey(segment sponsorblock.Segment) string {
    if segment.UUID != "" { return segment.UUID }
    return fmt.Sprintf("%s:%.3f:%.3f", segment.Category, segment.Start, segment.End)
}

func (m Model) nextSponsorHighlight() string {
    if !m.cfg.SponsorBlockEnabled || m.sponsorSegmentVideoID == "" || m.playingVid == nil || m.sponsorSegmentVideoID != m.playingVid.ID {
        return ""
    }
    for _, segment := range m.sponsorSegments {
        if segment.End <= m.npPos { continue }
        if segment.Start <= m.npPos && m.npPos < segment.End {
            return fmt.Sprintf("SB NOW: %s; Ctrl+J skips to %s", segment.Category, formatDuration(segment.End))
        }
        return fmt.Sprintf("SB next: %s at %s-%s; Ctrl+J to skip when it starts", segment.Category, formatDuration(segment.Start), formatDuration(segment.End))
    }
    return ""
}
