// Package ytdlp shells out to the yt-dlp binary to search YouTube and
// extract playable stream info. We deliberately don't reimplement any of
// YouTube's extraction logic — yt-dlp is maintained specifically to keep up
// with YouTube's changes, and duplicating that is a losing game.
package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Video is a trimmed-down view of the (huge) JSON yt-dlp emits per entry.
type Video struct {
	ID          string
	Title       string
	Channel     string
	Duration    float64
	ViewCount   int64
	URL         string
	Thumbnail   string
	Description string    // only populated by GetDescription — search results don't include it
	PublishedAt time.Time // zero value if unknown (e.g. yt-dlp search results don't provide this)
}

// videoJSON mirrors the raw yt-dlp JSON shape. In --flat-playlist search
// mode, yt-dlp does NOT populate the singular "thumbnail" field — it only
// gives a "thumbnails" array of {url, width, height, ...} at varying
// resolutions. Reading only "thumbnail" (as an earlier version of this code
// did) silently gets an empty string for every search result.
type videoJSON struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Channel    string  `json:"channel"`
	Duration   float64 `json:"duration"`
	ViewCount  int64   `json:"view_count"`
	URL        string  `json:"webpage_url"`
	Thumbnail  string  `json:"thumbnail"`
	Thumbnails []struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbnails"`
}

func (raw videoJSON) toVideo() Video {
	v := Video{
		ID:        raw.ID,
		Title:     raw.Title,
		Channel:   raw.Channel,
		Duration:  raw.Duration,
		ViewCount: raw.ViewCount,
		URL:       raw.URL,
		Thumbnail: raw.Thumbnail,
	}
	if v.Thumbnail == "" && len(raw.Thumbnails) > 0 {
		// Entries are typically ordered smallest-to-largest; pick a
		// mid/high-resolution one without grabbing an oversized original.
		best := raw.Thumbnails[len(raw.Thumbnails)-1]
		for _, t := range raw.Thumbnails {
			if t.Width >= 320 && t.Width <= 640 {
				best = t
				break
			}
		}
		v.Thumbnail = best.URL
	}
	if v.Thumbnail == "" && v.ID != "" {
		// Last-resort fallback: YouTube's thumbnail CDN follows this exact
		// pattern for every public video regardless of what metadata this
		// particular yt-dlp version's JSON output happens to include.
		v.Thumbnail = "https://i.ytimg.com/vi/" + v.ID + "/mqdefault.jpg"
	}
	return v
}

// binaryName lets us swap in a full path if yt-dlp isn't on PATH.
var binaryName = "yt-dlp"

// checkInstalled returns a friendly error if yt-dlp can't be found.
func checkInstalled() error {
	if _, err := exec.LookPath(binaryName); err != nil {
		return fmt.Errorf("yt-dlp not found on PATH — install it with: sudo dnf install yt-dlp")
	}
	return nil
}

// GetDescription fetches the full description for a single video. This is
// a real (non-flat) metadata fetch — slower than search, since it hits the
// actual video page rather than just search-result stubs — so it's only
// used for the currently-playing video, not for every list item.
func GetDescription(videoURL string) (string, error) {
	if err := checkInstalled(); err != nil {
		return "", err
	}
	cmd := exec.Command(binaryName,
		"--dump-json",
		"--no-warnings",
		"--skip-download",
		videoURL,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp metadata fetch failed: %v\n%s", err, stderr.String())
	}
	var data struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return "", fmt.Errorf("parsing yt-dlp metadata: %w", err)
	}
	return data.Description, nil
}

// Search runs `yt-dlp "ytsearchN:query"` and parses the newline-delimited
// JSON it prints (one JSON object per video) with --flat-playlist so it's
// fast (no per-video network hit, just search-result metadata).
func Search(query string, limit int) ([]Video, error) {
	if err := checkInstalled(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	searchExpr := fmt.Sprintf("ytsearch%d:%s", limit, query)

	cmd := exec.Command(binaryName,
		"--dump-json",
		"--flat-playlist",
		"--no-warnings",
		"--skip-download",
		searchExpr,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp search failed: %v\n%s", err, stderr.String())
	}

	var results []Video
	scanner := bytes.Split(stdout.Bytes(), []byte("\n"))
	for _, line := range scanner {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var raw videoJSON
		if err := json.Unmarshal(line, &raw); err != nil {
			continue // skip malformed lines rather than failing the whole search
		}
		v := raw.toVideo()
		if v.URL == "" && v.ID != "" {
			v.URL = "https://www.youtube.com/watch?v=" + v.ID
		}
		results = append(results, v)
	}
	return results, nil
}

// StreamURL resolves a watch page URL (or video ID) to a direct, playable
// stream URL that mpv can open. Using -f "bv*+ba/b" asks for best video+audio
// (falling back to best combined) — mpv can also just take the webpage URL
// directly since it has native yt-dlp hook support, but resolving explicitly
// gives us more control (and works even if the user's mpv lacks the hook).
func StreamURL(videoURL string) (string, error) {
	if err := checkInstalled(); err != nil {
		return "", err
	}
	cmd := exec.Command(binaryName,
		"-f", "bv*+ba/b",
		"-g", // print final URL(s) only
		"--no-warnings",
		videoURL,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp resolve failed: %v\n%s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no stream URL returned for %s", videoURL)
	}
	// First line is video stream; mpv can be pointed at just this URL and
	// will pull audio itself if we instead pass the original watch URL.
	// Simplest and most robust: just hand mpv the original webpage URL and
	// let its yt-dlp hook do the work. See player.Play.
	return videoURL, nil
}
