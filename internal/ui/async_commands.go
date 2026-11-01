package ui

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/player"
	"github.com/SakibShahariar/ytui/internal/sponsorblock"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func loadDirectURLCmd(videoURL string) tea.Cmd {
	return loadDirectURLWith(videoURL, ytdlp.GetVideo)
}

// loadDirectURLWith isolates metadata lookup from the Bubble Tea command so
// tests can exercise direct-URL message handling without invoking yt-dlp.
func loadDirectURLWith(videoURL string, getVideo func(string) (ytdlp.Video, error)) tea.Cmd {
	return func() tea.Msg {
		video, err := getVideo(videoURL)
		return directURLMsg{video: video, err: err}
	}
}

func doSearch(query string) tea.Cmd {
	if ytdlp.IsPlaylistURL(query) { return loadPlaylistCmd(query) }
	return doSearchWith(query, ytdlp.Search)
}

func loadSubtitleTracksCmd(video ytdlp.Video) tea.Cmd {
	return loadSubtitleTracksWith(video, ytdlp.GetSubtitleTracks)
}

func loadSubtitleTracksWith(video ytdlp.Video, getTracks func(string) ([]ytdlp.SubtitleTrack, error)) tea.Cmd {
	return func() tea.Msg {
		tracks, err := getTracks(video.URL)
		return subtitleTracksMsg{video: video, tracks: tracks, err: err}
	}
}

func loadPlaylistCmd(playlistURL string) tea.Cmd {
	return loadPlaylistWith(playlistURL, ytdlp.GetPlaylist)
}

func loadPlaylistWith(playlistURL string, load func(string) (string, []ytdlp.Video, error)) tea.Cmd {
	return func() tea.Msg {
		title, videos, err := load(playlistURL)
		return searchResultsMsg{results: videos, err: err, playlistTitle: title}
	}
}

// doSearchWith keeps the asynchronous search command testable without
// requiring yt-dlp or network access. Production uses ytdlp.Search; tests
// supply a deterministic function and inspect the resulting Bubble Tea message.
func doSearchWith(query string, search func(string, int) ([]ytdlp.Video, error)) tea.Cmd {
	return func() tea.Msg {
		results, err := search(query, 25)
		return searchResultsMsg{results: results, err: err}
	}
}

type playbackExitedMsg struct{ p playbackSession }

func waitForPlaybackExitCmd(p playbackSession) tea.Cmd {
	return func() tea.Msg {
		_ = p.Wait()
		return playbackExitedMsg{p: p}
	}
}

func startPlayback(url string, audioOnly bool, resumePos float64, subtitleLanguage ...string) tea.Cmd {
	return startPlaybackWith(url, audioOnly, resumePos, func(url string, audioOnly bool) (playbackSession, error) {
		p, err := player.Play(url, audioOnly, subtitleLanguage...)
		if p == nil {
			return nil, err
		}
		return p, err
	})
}

// startPlaybackWith keeps process creation injectable, allowing the playback
// startup and resume-seek flow to be tested without launching mpv.
func startPlaybackWith(url string, audioOnly bool, resumePos float64, play func(string, bool) (playbackSession, error)) tea.Cmd {
	return func() tea.Msg {
		p, err := play(url, audioOnly)
		if err == nil && p != nil && resumePos > 0 {
			// The IPC socket can become available before the media is loaded.
			// Wait for duration to become queryable before attempting the seek,
			// rather than relying on a fixed delay that varies by network speed.
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, durationErr := p.Duration(); durationErr == nil {
					if seekErr := p.SeekAbsolute(resumePos); seekErr == nil {
						break
					}
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return playbackStartedMsg{p: p, err: err}
	}
}

// positionTickCmd periodically saves the current playback position to
// history while a video is playing, not just on explicit stop/quit — so
// resume still works even if mpv gets closed some other way (its own 'q'
// key, closing its window, a crash) that we don't otherwise hear about.
func positionTickCmd() tea.Cmd {
	return tea.Tick(20*time.Second, func(time.Time) tea.Msg {
		return positionTickMsg{}
	})
}

// nowPlayingTickCmd drives the live Now Playing bar (position, duration,
// volume, pause state). Ticks much faster than positionTickCmd since a
// progress bar needs to feel live, whereas resume-position only needs
// occasional saves.
func nowPlayingTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return nowPlayingTickMsg{}
	})
}

func fetchDescriptionCmd(videoID, videoURL string) tea.Cmd {
	return fetchDescriptionWith(videoID, videoURL, ytdlp.GetDescription)
}

// fetchDescriptionWith makes the description command deterministic in tests.
func fetchDescriptionWith(videoID, videoURL string, getDescription func(string) (string, error)) tea.Cmd {
	return func() tea.Msg {
		desc, err := getDescription(videoURL)
		return videoDescriptionMsg{videoID: videoID, desc: desc, err: err}
	}
}

// downloadsTickCmd drives the live progress bar on the Downloads tab.
// Ticks fairly often (progress should feel live) but only while something
// is actually queued/downloading — see the downloadsTickMsg handler.
func downloadsTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return downloadsTickMsg{}
	})
}


const (
	maxTranscriptCacheEntries = 32
	maxTranscriptCacheBytes = 4 << 20
)

type transcriptCacheKey struct {
	videoID string
	language string
}

type transcriptCacheEntry struct {
	key transcriptCacheKey
	text string
}

type transcriptCache struct {
	mu sync.Mutex
	entries map[transcriptCacheKey]*list.Element
	order *list.List
	bytes int
	maxEntries int
	maxBytes int
}

func newTranscriptCache(maxEntries, maxBytes int) *transcriptCache {
	return &transcriptCache{
		entries: make(map[transcriptCacheKey]*list.Element),
		order: list.New(),
		maxEntries: maxEntries,
		maxBytes: maxBytes,
	}
}

func (cache *transcriptCache) get(key transcriptCacheKey) (string, bool) {
	if cache == nil || key.videoID == "" { return "", false }
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element, ok := cache.entries[key]
	if !ok { return "", false }
	cache.order.MoveToFront(element)
	return element.Value.(transcriptCacheEntry).text, true
}

func (cache *transcriptCache) put(key transcriptCacheKey, text string) {
	if cache == nil || key.videoID == "" || text == "" || len(text) > cache.maxBytes || cache.maxEntries < 1 || cache.maxBytes < 1 { return }
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if old, ok := cache.entries[key]; ok {
		cache.bytes -= len(old.Value.(transcriptCacheEntry).text)
		cache.order.Remove(old)
		delete(cache.entries, key)
	}
	element := cache.order.PushFront(transcriptCacheEntry{key: key, text: text})
	cache.entries[key] = element
	cache.bytes += len(text)
	for cache.order.Len() > cache.maxEntries || cache.bytes > cache.maxBytes {
		oldest := cache.order.Back()
		entry := oldest.Value.(transcriptCacheEntry)
		cache.bytes -= len(entry.text)
		delete(cache.entries, entry.key)
		cache.order.Remove(oldest)
	}
}

var sharedTranscriptCache = newTranscriptCache(maxTranscriptCacheEntries, maxTranscriptCacheBytes)

func transcriptCacheLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" { return "en.*,en" }
	return language
}

func loadTranscriptCmd(video ytdlp.Video, language string) tea.Cmd {
	return loadTranscriptWith(video, language, ytdlp.GetTranscript)
}

func loadTranscriptWith(video ytdlp.Video, language string, getTranscript func(string, string) (string, error)) tea.Cmd {
	return func() tea.Msg {
		key := transcriptCacheKey{videoID: video.ID, language: transcriptCacheLanguage(language)}
		if transcript, ok := sharedTranscriptCache.get(key); ok {
			return transcriptMsg{video: video, text: transcript}
		}
		transcript, err := getTranscript(video.URL, language)
		if err == nil {
			sharedTranscriptCache.put(key, transcript)
		}
		return transcriptMsg{video: video, text: transcript, err: err}
	}
}

func loadSponsorSegmentsCmd(videoID string) tea.Cmd {
	return loadSponsorSegmentsWith(videoID, sponsorblock.Categories, sponsorblock.Fetch)
}

func loadSponsorSegmentsWith(videoID string, categories []string, fetch func(context.Context, string, []string) ([]sponsorblock.Segment, error)) tea.Cmd {
	return func() tea.Msg {
		segments, err := fetch(context.Background(), videoID, categories)
		return sponsorSegmentsMsg{videoID: videoID, segments: segments, err: err}
	}
}
