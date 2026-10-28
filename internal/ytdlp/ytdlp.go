// Package ytdlp shells out to the yt-dlp binary to search YouTube and
// extract playable stream info. We deliberately don't reimplement any of
// YouTube's extraction logic — yt-dlp is maintained specifically to keep up
// with YouTube's changes, and duplicating that is a losing game.
package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sort"
	"time"
)

// Video is a trimmed-down view of the (huge) JSON yt-dlp emits per entry.
type Video struct {
	ID          string
	Title       string
	Channel     string
	ChannelID   string
	ChannelURL  string
	Duration    float64
	ViewCount   int64
	URL         string
	Thumbnail   string
	Description string    // only populated by GetDescription — search results don't include it
	PlaylistIndex int // one-based position when loaded from a playlist
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
	ChannelID  string  `json:"channel_id"`
	ChannelURL string  `json:"channel_url"`
	UploaderID string  `json:"uploader_id"`
	UploaderURL string `json:"uploader_url"`
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
	channelURL := raw.ChannelURL
	if channelURL == "" {
		channelURL = raw.UploaderURL
	}
	channelID := raw.ChannelID
	if channelID == "" {
		channelID = raw.UploaderID
	}
	if channelID == "" {
		channelID = channelURL
	}
	v := Video{
		ID:        raw.ID,
		Title:     raw.Title,
		Channel:   raw.Channel,
		ChannelID: channelID,
		ChannelURL: channelURL,
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

// StreamURL keeps the watch-page URL intact for mpv's yt-dlp integration.
// Resolving it here would perform a network request whose result is discarded;
// mpv must receive the original URL so it can select compatible streams itself.
// GetVideo extracts metadata for one URL without downloading its media.
func GetVideo(videoURL string) (Video, error) {
	if err := checkInstalled(); err != nil {
		return Video{}, err
	}
	cmd := exec.Command(binaryName, "--dump-single-json", "--no-warnings", "--skip-download", "--no-playlist", videoURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Video{}, fmt.Errorf("yt-dlp could not read URL: %s", message)
	}
	var raw videoJSON
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return Video{}, fmt.Errorf("parse yt-dlp metadata: %w", err)
	}
	video := raw.toVideo()
	if video.ID == "" {
		video.ID = videoURL
	}
	if video.Title == "" {
		video.Title = videoURL
	}
	if video.Channel == "" {
		video.Channel = "Direct URL"
	}
	if video.URL == "" {
		video.URL = videoURL
	}
	return video, nil
}

func StreamURL(videoURL string) (string, error) {
	if strings.TrimSpace(videoURL) == "" {
		return "", fmt.Errorf("video URL is empty")
	}
	return videoURL, nil
}


type playlistJSON struct {
	Title string `json:"title"`
	Entries []videoJSON `json:"entries"`
}

func IsPlaylistURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") { return false }
	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && !strings.HasSuffix(host, ".youtube.com") && host != "youtu.be" { return false }
	return strings.TrimSpace(u.Query().Get("list")) != ""
}

func GetPlaylist(playlistURL string) (string, []Video, error) {
	if !IsPlaylistURL(playlistURL) { return "", nil, fmt.Errorf("not a YouTube playlist URL") }
	if err := checkInstalled(); err != nil { return "", nil, err }
	cmd := exec.Command(binaryName, "--dump-single-json", "--flat-playlist", "--yes-playlist", "--no-warnings", "--skip-download", playlistURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" { message = err.Error() }
		return "", nil, fmt.Errorf("yt-dlp could not load playlist: %s", message)
	}
	return parsePlaylistJSON(stdout.Bytes())
}

func parsePlaylistJSON(data []byte) (string, []Video, error) {
	var raw playlistJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", nil, fmt.Errorf("parse playlist metadata: %w", err)
	}
	title := strings.TrimSpace(raw.Title)
	if title == "" { title = "YouTube playlist" }
	videos := make([]Video, 0, len(raw.Entries))
	for _, entry := range raw.Entries {
		if strings.TrimSpace(entry.ID) == "" { continue }
		video := entry.toVideo()
		if video.URL == "" { video.URL = "https://www.youtube.com/watch?v=" + video.ID }
		video.PlaylistIndex = len(videos) + 1
		videos = append(videos, video)
	}
	if len(videos) == 0 { return title, nil, fmt.Errorf("playlist contains no accessible videos") }
	return title, videos, nil
}


type SubtitleTrack struct {
	Language string
	Name string
	Automatic bool
}

type subtitleMetadataJSON struct {
	Subtitles map[string][]struct { Name string `json:"name"` } `json:"subtitles"`
	AutomaticCaptions map[string][]struct { Name string `json:"name"` } `json:"automatic_captions"`
}

// GetSubtitleTracks lists manually uploaded and automatic caption languages
// without downloading media. Automatic captions are marked for the UI.
func GetSubtitleTracks(videoURL string) ([]SubtitleTrack, error) {
	if strings.TrimSpace(videoURL) == "" { return nil, fmt.Errorf("video URL is empty") }
	if err := checkInstalled(); err != nil { return nil, err }
	cmd := exec.Command(binaryName, "--dump-single-json", "--no-warnings", "--skip-download", "--no-playlist", videoURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" { message = err.Error() }
		return nil, fmt.Errorf("yt-dlp could not list subtitles: %s", message)
	}
	var metadata subtitleMetadataJSON
	if err := json.Unmarshal(stdout.Bytes(), &metadata); err != nil { return nil, fmt.Errorf("parse subtitle metadata: %w", err) }
	tracks := make([]SubtitleTrack, 0, len(metadata.Subtitles)+len(metadata.AutomaticCaptions))
	seen := make(map[string]bool)
	appendTracks := func(groups map[string][]struct { Name string `json:"name"` }, automatic bool) {
		langs := make([]string, 0, len(groups))
		for lang := range groups { langs = append(langs, lang) }
		sort.Strings(langs)
		for _, lang := range langs {
			if seen[lang] { continue }
			name := ""
			if len(groups[lang]) > 0 { name = strings.TrimSpace(groups[lang][0].Name) }
			if name == "" { name = lang }
			tracks = append(tracks, SubtitleTrack{Language: lang, Name: name, Automatic: automatic})
			seen[lang] = true
		}
	}
	appendTracks(metadata.Subtitles, false)
	appendTracks(metadata.AutomaticCaptions, true)
	if len(tracks) == 0 { return nil, fmt.Errorf("no subtitles or automatic captions are available") }
	return tracks, nil
}
