// Package ytapi calls the YouTube Data API v3 directly over REST (no
// generated client library — the official Go client for this API is a
// very large dependency for the handful of endpoints ytui actually needs).
// All calls here require an authenticated client from internal/auth.
package ytapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

const apiBase = "https://www.googleapis.com/youtube/v3"

// Subscription is a channel the user is subscribed to on YouTube.
type Subscription struct {
	ChannelID string
	Title     string
	Thumbnail string
}

// Playlist is a playlist owned by the user (Watch Later and Liked are
// special-cased by YouTube and not returned here — Liked videos are
// fetched separately via LikedVideos, which uses the dedicated endpoint).
type Playlist struct {
	ID        string
	Title     string
	ItemCount int
	Thumbnail string
}

func get(client *http.Client, endpoint string, params url.Values) (json.RawMessage, error) {
	u := apiBase + "/" + endpoint + "?" + params.Encode()
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("YouTube API request failed: %w", err)
	}
	defer resp.Body.Close()

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("YouTube API returned unreadable response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("YouTube API: %s", msg)
	}
	return raw, nil
}

// ChannelUploadsPlaylistID resolves a channel ID to its "uploads" playlist
// ID — every channel has one, and it behaves like a normal playlist you
// can list videos from via PlaylistVideos, giving a "recent uploads" feed
// for a subscribed channel without needing a separate activities/search call.
var uploadsPlaylistCache sync.Map

func ChannelUploadsPlaylistID(client *http.Client, channelID string) (string, error) {
	if cached, ok := uploadsPlaylistCache.Load(channelID); ok {
		return cached.(string), nil
	}
	params := url.Values{
		"part": {"contentDetails"},
		"id":   {channelID},
	}
	raw, err := get(client, "channels", params)
	if err != nil {
		return "", err
	}
	var page struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return "", fmt.Errorf("parsing channel response: %w", err)
	}
	if len(page.Items) == 0 {
		return "", fmt.Errorf("channel not found")
	}
	uploads := page.Items[0].ContentDetails.RelatedPlaylists.Uploads
	if uploads == "" {
		return "", fmt.Errorf("channel has no uploads playlist")
	}
	// Uploads playlist IDs are stable for a channel. Cache them for this
	// process so refreshing Home doesn't spend another channels.list quota
	// unit for every subscription each time.
	uploadsPlaylistCache.Store(channelID, uploads)
	return uploads, nil
}

// HomeFeed aggregates recent uploads across every given subscription into
// one feed, sorted most-recent-first — like YouTube's actual homepage,
// built from data we already have calls for (ChannelVideos), just fetched
// concurrently across channels instead of one at a time (which would be
// painfully slow for anyone subscribed to more than a handful of channels).
//
// perChannelLimit caps how many videos we pull per channel (e.g. 5) —
// getting a full history from every channel just to show the ~30 newest
// overall would be wasteful.
func HomeFeed(client *http.Client, subs []Subscription, perChannelLimit int) ([]ytdlp.Video, int, error) {
	const maxConcurrent = 8

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []playlistItemStub
	var firstErr error
	failed := 0

	for _, sub := range subs {
		wg.Add(1)
		go func(channelID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			playlistID, err := ChannelUploadsPlaylistID(client, channelID)
			if err != nil {
				mu.Lock()
				failed++
				if firstErr == nil { firstErr = err }
				mu.Unlock()
				return
			}
			stubs, err := playlistVideoStubs(client, playlistID, perChannelLimit)
			if err != nil {
				mu.Lock()
				failed++
				if firstErr == nil { firstErr = err }
				mu.Unlock()
				return
			}
			mu.Lock()
			all = append(all, stubs...)
			mu.Unlock()
		}(sub.ChannelID)
	}
	wg.Wait()

	durations, views, _ := videoDetailsPartial(client, uniquePlaylistVideoIDs(all))
	videos := videosFromPlaylistStubs(all, durations, views)
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].PublishedAt.After(videos[j].PublishedAt)
	})
	if len(videos) == 0 && firstErr != nil {
		return nil, failed, firstErr
	}
	return videos, failed, nil
}

// Subscriptions fetches all of the user's subscriptions, paginating until
// exhausted. Order is alphabetical, matching the YouTube app's default.
func Subscriptions(client *http.Client) ([]Subscription, error) {
	var out []Subscription
	pageToken := ""
	for {
		params := url.Values{
			"part":       {"snippet"},
			"mine":       {"true"},
			"maxResults": {"50"},
			"order":      {"alphabetical"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := get(client, "subscriptions", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				Snippet struct {
					Title      string `json:"title"`
					Thumbnails struct {
						Default struct {
							URL string `json:"url"`
						} `json:"default"`
					} `json:"thumbnails"`
					ResourceID struct {
						ChannelID string `json:"channelId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parsing subscriptions response: %w", err)
		}
		for _, it := range page.Items {
			out = append(out, Subscription{
				ChannelID: it.Snippet.ResourceID.ChannelID,
				Title:     it.Snippet.Title,
				Thumbnail: it.Snippet.Thumbnails.Default.URL,
			})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return out, nil
}

// Playlists fetches all playlists owned by the user.
func Playlists(client *http.Client) ([]Playlist, error) {
	var out []Playlist
	pageToken := ""
	for {
		params := url.Values{
			"part":       {"snippet,contentDetails"},
			"mine":       {"true"},
			"maxResults": {"50"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := get(client, "playlists", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title      string `json:"title"`
					Thumbnails struct {
						Medium struct {
							URL string `json:"url"`
						} `json:"medium"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				ContentDetails struct {
					ItemCount int `json:"itemCount"`
				} `json:"contentDetails"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parsing playlists response: %w", err)
		}
		for _, it := range page.Items {
			out = append(out, Playlist{
				ID:        it.ID,
				Title:     it.Snippet.Title,
				ItemCount: it.ContentDetails.ItemCount,
				Thumbnail: it.Snippet.Thumbnails.Medium.URL,
			})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return out, nil
}

type playlistItemStub struct {
	videoID     string
	title       string
	channel     string
	thumb       string
	publishedAt string
}

// ChannelVideos fetches a channel's recent uploads (its "uploads" playlist,
// resolved automatically). Combines ChannelUploadsPlaylistID + PlaylistVideos
// so callers don't need to juggle the two-step lookup themselves.
func ChannelVideos(client *http.Client, channelID string) ([]ytdlp.Video, error) {
	playlistID, err := ChannelUploadsPlaylistID(client, channelID)
	if err != nil {
		return nil, err
	}
	return PlaylistVideos(client, playlistID)
}

// PlaylistVideos fetches every video in a playlist, with duration/view
// count filled in via a follow-up batched videos.list call. For just the
// first page (e.g. a home-feed use case that only wants the few latest
// items), use PlaylistVideosLimited instead — much cheaper than paginating
// through an entire playlist.
func PlaylistVideos(client *http.Client, playlistID string) ([]ytdlp.Video, error) {
	return playlistVideos(client, playlistID, 0)
}

// PlaylistVideosLimited fetches at most `limit` videos (a single page,
// most-recently-added first) without paginating further — used by the
// home feed, which only needs a handful of recent uploads per channel,
// not each channel's entire upload history.
func PlaylistVideosLimited(client *http.Client, playlistID string, limit int) ([]ytdlp.Video, error) {
	return playlistVideos(client, playlistID, limit)
}

func playlistVideos(client *http.Client, playlistID string, limit int) ([]ytdlp.Video, error) {
	stubs, err := playlistVideoStubs(client, playlistID, limit)
	if err != nil {
		return nil, err
	}
	durations, views, err := videoDetails(client, uniquePlaylistVideoIDs(stubs))
	if err != nil {
		// Non-fatal — show the list without duration/views rather than
		// failing the whole playlist load over a secondary call.
		durations, views = map[string]float64{}, map[string]int64{}
	}
	return videosFromPlaylistStubs(stubs, durations, views), nil
}

func playlistVideoStubs(client *http.Client, playlistID string, limit int) ([]playlistItemStub, error) {
	var stubs []playlistItemStub
	pageToken := ""
	maxResults := "50"
	if limit > 0 && limit < 50 {
		maxResults = fmt.Sprint(limit)
	}
	for {
		params := url.Values{
			"part":       {"snippet"},
			"playlistId": {playlistID},
			"maxResults": {maxResults},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := get(client, "playlistItems", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				Snippet struct {
					Title             string `json:"title"`
					VideoOwnerChannel string `json:"videoOwnerChannelTitle"`
					PublishedAt       string `json:"publishedAt"`
					Thumbnails        struct {
						Medium struct {
							URL string `json:"url"`
						} `json:"medium"`
					} `json:"thumbnails"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parsing playlist items response: %w", err)
		}
		for _, item := range page.Items {
			if item.Snippet.ResourceID.VideoID == "" {
				continue
			}
			stubs = append(stubs, playlistItemStub{
				videoID: item.Snippet.ResourceID.VideoID,
				title: item.Snippet.Title,
				channel: item.Snippet.VideoOwnerChannel,
				thumb: item.Snippet.Thumbnails.Medium.URL,
				publishedAt: item.Snippet.PublishedAt,
			})
			if limit > 0 && len(stubs) >= limit { break }
		}
		if page.NextPageToken == "" || (limit > 0 && len(stubs) >= limit) { break }
		pageToken = page.NextPageToken
	}
	return stubs, nil
}

func uniquePlaylistVideoIDs(stubs []playlistItemStub) []string {
	seen := make(map[string]struct{}, len(stubs))
	ids := make([]string, 0, len(stubs))
	for _, stub := range stubs {
		if _, ok := seen[stub.videoID]; ok { continue }
		seen[stub.videoID] = struct{}{}
		ids = append(ids, stub.videoID)
	}
	return ids
}

func videosFromPlaylistStubs(stubs []playlistItemStub, durations map[string]float64, views map[string]int64) []ytdlp.Video {
	out := make([]ytdlp.Video, 0, len(stubs))
	for _, stub := range stubs {
		published, _ := time.Parse(time.RFC3339, stub.publishedAt)
		out = append(out, ytdlp.Video{
			ID: stub.videoID,
			Title: stub.title,
			Channel: stub.channel,
			Thumbnail: stub.thumb,
			URL: "https://www.youtube.com/watch?v=" + stub.videoID,
			Duration: durations[stub.videoID],
			ViewCount: views[stub.videoID],
			PublishedAt: published,
		})
	}
	return out
}

// LikedVideos fetches videos the user has liked, via the dedicated
// myRating=like endpoint (liked videos aren't a normal playlist you can
// list items from the same way as user-created playlists).
func LikedVideos(client *http.Client) ([]ytdlp.Video, error) {
	var out []ytdlp.Video
	pageToken := ""
	for {
		params := url.Values{
			"part":       {"snippet,contentDetails,statistics"},
			"myRating":   {"like"},
			"maxResults": {"50"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := get(client, "videos", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title        string `json:"title"`
					ChannelTitle string `json:"channelTitle"`
					Thumbnails   struct {
						Medium struct {
							URL string `json:"url"`
						} `json:"medium"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Statistics struct {
					ViewCount string `json:"viewCount"`
				} `json:"statistics"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parsing liked videos response: %w", err)
		}
		for _, it := range page.Items {
			views, _ := strconv.ParseInt(it.Statistics.ViewCount, 10, 64)
			out = append(out, ytdlp.Video{
				ID:        it.ID,
				Title:     it.Snippet.Title,
				Channel:   it.Snippet.ChannelTitle,
				Thumbnail: it.Snippet.Thumbnails.Medium.URL,
				URL:       "https://www.youtube.com/watch?v=" + it.ID,
				Duration:  parseISO8601Duration(it.ContentDetails.Duration),
				ViewCount: views,
			})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return out, nil
}

// videoDetails batch-fetches duration and view count for up to many video
// IDs, 50 at a time (the API's per-request limit for the id parameter).
func videoDetails(client *http.Client, ids []string) (map[string]float64, map[string]int64, error) {
	durations := map[string]float64{}
	views := map[string]int64{}
	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]
		if len(batch) == 0 {
			continue
		}
		params := url.Values{
			"part": {"contentDetails,statistics"},
			"id":   {strings.Join(batch, ",")},
		}
		raw, err := get(client, "videos", params)
		if err != nil {
			return nil, nil, err
		}
		var page struct {
			Items []struct {
				ID             string `json:"id"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Statistics struct {
					ViewCount string `json:"viewCount"`
				} `json:"statistics"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, nil, fmt.Errorf("parsing video details response: %w", err)
		}
		for _, it := range page.Items {
			durations[it.ID] = parseISO8601Duration(it.ContentDetails.Duration)
			v, _ := strconv.ParseInt(it.Statistics.ViewCount, 10, 64)
			views[it.ID] = v
		}
	}
	return durations, views, nil
}

var iso8601DurationRe = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISO8601Duration converts YouTube's duration format ("PT4M13S") to
// seconds. Returns 0 for anything it doesn't recognize (e.g. live streams,
// which report duration as "P0D").
func parseISO8601Duration(s string) float64 {
	m := iso8601DurationRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	return float64(h*3600 + min*60 + sec)
}
