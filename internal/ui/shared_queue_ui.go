package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/sharedqueue"
)

func loadSharedQueueCmd(manual bool) tea.Cmd {
	return func() tea.Msg {
		items, err := sharedqueue.Read()
		return sharedQueueMsg{items: items, err: err, manual: manual}
	}
}

func ackSharedQueueCmd(ids []string, count int) tea.Cmd {
	copied := append([]string(nil), ids...)
	return func() tea.Msg {
		return sharedQueueAckMsg{count: count, err: sharedqueue.Ack(copied)}
	}
}

func sharedQueueOptions(item sharedqueue.Item) downloader.Options {
	quality := strings.ToLower(strings.TrimSpace(item.Quality))
	opts := downloader.Options{Format: "bestvideo+bestaudio/best", QualityLabel: "Best quality (video+audio)"}
	switch quality {
	case "1080p":
		opts.Format = "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
		opts.QualityLabel = "1080p"
	case "720p":
		opts.Format = "bestvideo[height<=720]+bestaudio/best[height<=720]"
		opts.QualityLabel = "720p"
	case "480p":
		opts.Format = "bestvideo[height<=480]+bestaudio/best[height<=480]"
		opts.QualityLabel = "480p"
	}
	if item.Mode == "audio" {
		opts.ExtractAudio = true
		format := strings.ToLower(strings.TrimSpace(item.AudioFormat))
		switch format {
		case "", "best", "mp3", "m4a", "opus", "wav", "flac", "alac", "aac", "vorbis":
			opts.AudioFormat = format
		default:
			opts.AudioFormat = "mp3"
		}
		if opts.AudioFormat == "" { opts.AudioFormat = "best" }
		opts.QualityLabel = "Audio only (" + opts.AudioFormat + ")"
	}
	return opts
}

func (m Model) importSharedQueueItems(items []sharedqueue.Item) (tea.Model, tea.Cmd) {
	if len(items) == 0 {
		m.statusMsg = "shared queue is empty"
		return m, nil
	}
	if m.downloads == nil {
		m.statusMsg = "shared queue has items, but downloads are unavailable; items left in the inbox"
		return m, nil
	}
	wasActive := m.downloads.Active()
	ackIDs := make([]string, 0, len(items))
	imported := 0
	for _, item := range items {
		d := m.downloads.Enqueue("shared:"+item.ID, item.Title, "", item.URL, sharedQueueOptions(item))
		if d.Status == downloader.StatusCanceled { continue }
		ackIDs = append(ackIDs, item.ID)
		imported++
	}
	if len(ackIDs) == 0 {
		m.statusMsg = "shared queue items were not accepted; they remain in the inbox"
		return m, nil
	}
	m.statusMsg = "importing " + formatCount(imported, "shared queue item") + " from Ytrawl…"
	cmds := []tea.Cmd{ackSharedQueueCmd(ackIDs, imported)}
	if !wasActive && m.downloads.Active() { cmds = append(cmds, downloadsTickCmd()) }
	return m, tea.Batch(cmds...)
}

func formatCount(count int, noun string) string {
	if count == 1 { return "1 " + noun }
	return strconv.Itoa(count) + " " + noun + "s"
}
