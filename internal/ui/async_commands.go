package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/player"
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
	return doSearchWith(query, ytdlp.Search)
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

func startPlayback(url string, audioOnly bool, resumePos float64) tea.Cmd {
	return func() tea.Msg {
		p, err := player.Play(url, audioOnly)
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
		// Avoid storing a typed nil *player.Player in the interface: a typed
		// nil inside an interface compares non-nil and would look like playback.
		var session playbackSession
		if p != nil {
			session = p
		}
		return playbackStartedMsg{p: session, err: err}
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

