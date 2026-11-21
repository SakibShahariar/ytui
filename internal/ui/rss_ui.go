package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SakibShahariar/ytui/internal/rss"
	"github.com/SakibShahariar/ytui/internal/store"
)

type rssRefreshMsg struct {
	items    []rss.Item
	failures []string
}

func refreshRSSCmd(s *store.Store) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := rssRefreshMsg{}
		for _, feed := range s.RSSFeeds {
			_, items, err := rss.Fetch(ctx, nil, feed.URL)
			if err != nil {
				result.failures = append(result.failures, feed.Name+": "+err.Error())
				continue
			}
			for _, item := range items {
				item.Title = feed.Name + " — " + item.Title
				result.items = append(result.items, item)
			}
			_ = s.UpdateRSSFeed(feed.URL, time.Now(), "")
		}
		return result
	}
}

// maybeAutoRSSCmd schedules a background refresh only when configured and at
// least one feed is due. Zero interval leaves refresh fully on demand.
func (m Model) maybeAutoRSSCmd() tea.Cmd {
	if m.cfg.RSSRefreshIntervalHours <= 0 || m.store == nil {
		return nil
	}
	interval := time.Duration(m.cfg.RSSRefreshIntervalHours) * time.Hour
	for _, feed := range m.store.RSSFeeds {
		if feed.LastChecked.IsZero() || time.Since(feed.LastChecked) >= interval {
			return refreshRSSCmd(m.store)
		}
	}
	return nil
}

func (m Model) handleRSSRefresh(msg rssRefreshMsg) (tea.Model, tea.Cmd) {
	m.rssViewer = true
	m.rssItems = msg.items
	if len(msg.failures) > 0 {
		m.statusMsg = fmt.Sprintf("RSS refresh: %d item(s), %d feed(s) failed", len(msg.items), len(msg.failures))
	} else {
		m.statusMsg = fmt.Sprintf("RSS refresh: %d item(s)", len(msg.items))
	}
	return m, nil
}

func (m Model) rssViewerContent(width int) string {
	if width < 20 {
		width = 20
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("RSS subscriptions"),
		"",
	}
	if len(m.rssItems) == 0 {
		lines = append(lines, "No feed items loaded. Add feeds with: ytui rss add URL")
	} else {
		for _, item := range m.rssItems {
			when := ""
			if !item.Published.IsZero() {
				when = " (" + item.Published.Format("2006-01-02") + ")"
			}
			lines = append(lines, "  "+truncate(item.Title+when, width-4))
		}
	}
	lines = append(lines, "", "esc: close · add/remove feeds with 'ytui rss'")
	return strings.Join(lines, "\n")
}
