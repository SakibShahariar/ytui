package ui

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/library"
	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func (m Model) selectedVideo() (ytdlp.Video, bool) {
	item := m.list.SelectedItem()
	if item == nil {
		return ytdlp.Video{}, false
	}
	switch it := item.(type) {
	case videoItem:
		return it.v, true
	case watchLaterItem:
		return ytdlp.Video{ID: it.e.VideoID, Title: it.e.Title, Channel: it.e.Channel, URL: it.e.URL,
			Thumbnail: thumbnailFor(it.e.VideoID, it.e.Thumbnail)}, true
	case historyItem:
		return ytdlp.Video{ID: it.e.VideoID, Title: it.e.Title, Channel: it.e.Channel, URL: it.e.URL,
			Thumbnail: thumbnailFor(it.e.VideoID, it.e.Thumbnail)}, true
	}
	return ytdlp.Video{}, false
}

// thumbnailFor returns stored if it's non-empty, otherwise derives
// YouTube's standard thumbnail CDN URL directly from the video ID — this
// works for every video regardless of when the entry was saved, so older
// History/Watch Later entries (from before thumbnails were stored) still
// get a preview without needing any data migration.
func thumbnailFor(videoID, stored string) string {
	if stored != "" {
		return stored
	}
	if videoID == "" {
		return ""
	}
	return "https://i.ytimg.com/vi/" + videoID + "/mqdefault.jpg"
}

// saveResumePosition asks mpv for the current playback position and
// updates the video's history entry with it — so next time it's played,
// we can offer to resume where you left off. Best-effort: if mpv doesn't
// respond in time (e.g. it's already exiting), this just silently no-ops
// rather than blocking the quit/stop action.
func (m Model) saveResumePosition() {
	if m.nowPlaying == nil || m.playingVid == nil {
		return
	}
	pos, err := m.nowPlaying.Position()
	if err != nil || pos <= 0 {
		// mpv may already be exiting. Preserve the most recent position
		// observed by the UI rather than discarding all progress since the
		// previous periodic save.
		pos = m.npPos
	}
	if pos <= 0 {
		return
	}
	v := m.playingVid
	_ = m.store.AddHistory(store.HistoryEntry{
		VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL,
		Thumbnail: v.Thumbnail, PositionS: pos,
	})
}

func (m Model) playLocalFile(file library.File) (tea.Model, tea.Cmd) {
	if m.nowPlaying != nil {
		m.saveResumePosition()
		_ = m.nowPlaying.Quit()
	}
	m.playingLocal = true
	m.playingVid = &ytdlp.Video{ID: "local:" + file.Path, Title: file.Name, Channel: "Downloaded file", URL: file.Path}
	m.statusMsg = "starting mpv: " + file.Name
	return m, startPlayback(file.Path, m.audioOnly, 0)
}

func (m Model) playSelected() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	if m.nowPlaying != nil {
		m.saveResumePosition()
		_ = m.nowPlaying.Quit()
	}
	resumePos, resuming := m.store.FindHistoryPosition(v.ID)
	validPosition := resuming && !math.IsNaN(resumePos) && !math.IsInf(resumePos, 0) && resumePos > 0 &&
		(v.Duration <= 0 || resumePos < v.Duration)
	if validPosition {
		m.resumePrompt = &resumePrompt{video: v, position: resumePos}
		m.statusMsg = fmt.Sprintf("resume %s or start from beginning: %s", formatDuration(resumePos), v.Title)
		return m, nil
	}
	return m.startSelectedVideo(v, 0)
}

func (m Model) startSelectedVideo(v ytdlp.Video, resumePos float64) (tea.Model, tea.Cmd) {
	m.resumePrompt = nil
	m.playingLocal = false
	m.playingVid = &v
	if resumePos > 0 {
		m.statusMsg = fmt.Sprintf("starting mpv (resuming from %s): %s", formatDuration(resumePos), v.Title)
	} else {
		m.statusMsg = "starting mpv: " + v.Title
	}
	_ = m.store.AddHistory(store.HistoryEntry{VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL, Thumbnail: v.Thumbnail, PositionS: resumePos})
	return m, startPlayback(v.URL, m.audioOnly, resumePos)
}

// clearNowPlaying resets all Now Playing state — used both when we stop
// playback ourselves (ctrl+x) and when the 1s poll detects mpv is gone
// (video ended, closed some other way). If the person is currently
// viewing the Now Playing tab, navigates them away from it since it'd
// otherwise show stale/empty content while still being "selected".
func (m Model) clearNowPlaying() (tea.Model, tea.Cmd) {
	endedID := ""
	if m.playingVid != nil {
		endedID = m.playingVid.ID
	}
	shouldAutoplay := m.cfg.AutoplayNext && m.statusMsg != "playback stopped" && !m.playingLocal && endedID != ""
	m.nowPlaying = nil
	m.playingVid = nil
	m.playingLocal = false
	m.npPos, m.npDur, m.npVol, m.npPaused = 0, 0, 0, false
	m.npDescription = ""
	m.npDescriptionLoading = false
	if shouldAutoplay {
		if next, ok := m.nextWatchLaterVideo(endedID); ok {
			m.statusMsg = "autoplay next: " + next.Title
			return m.startSelectedVideo(next, 0)
		}
		if next, ok := m.nextCurrentListVideo(); ok {
			m.statusMsg = "autoplay next: " + next.Title
			return m.startSelectedVideo(next, 0)
		}
	}
	if m.activeTab == tabNowPlaying {
		m.activeTab = tabSearch
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
	}
	return m, tea.ClearScreen
}

// nextWatchLaterVideo prefers the item after the finished video when it is
// already in Watch Later; otherwise it starts at the queue's first item.
func (m Model) nextWatchLaterVideo(currentID string) (ytdlp.Video, bool) {
	entries := m.store.WatchLater
	start := 0
	for i, entry := range entries {
		if entry.VideoID == currentID {
			start = i + 1
			break
		}
	}
	if start >= len(entries) {
		return ytdlp.Video{}, false
	}
	entry := entries[start]
	return ytdlp.Video{ID: entry.VideoID, Title: entry.Title, Channel: entry.Channel, URL: entry.URL,
		Thumbnail: thumbnailFor(entry.VideoID, entry.Thumbnail)}, true
}

// nextCurrentListVideo advances from the current selection when the active
// list contains another playable video. Watch Later takes priority.
func (m Model) nextCurrentListVideo() (ytdlp.Video, bool) {
	items := m.list.Items()
	for i := m.list.Index() + 1; i < len(items); i++ {
		switch item := items[i].(type) {
		case videoItem:
			return item.v, true
		case watchLaterItem:
			return ytdlp.Video{ID: item.e.VideoID, Title: item.e.Title, Channel: item.e.Channel, URL: item.e.URL,
				Thumbnail: thumbnailFor(item.e.VideoID, item.e.Thumbnail)}, true
		case historyItem:
			return ytdlp.Video{ID: item.e.VideoID, Title: item.e.Title, Channel: item.e.Channel, URL: item.e.URL,
				Thumbnail: thumbnailFor(item.e.VideoID, item.e.Thumbnail)}, true
		}
	}
	return ytdlp.Video{}, false
}

