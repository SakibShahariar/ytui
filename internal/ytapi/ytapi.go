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

	"ytui/internal/ytdlp"
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
func ChannelUploadsPlaylistID(client *http.Client, channelID string) (string, error) {
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
func HomeFeed(client *http.Client, subs []Subscription, perChannelLimit int) ([]ytdlp.Video, error) {
	const maxConcurrent = 8

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []ytdlp.Video
	var firstErr error

	for _, sub := range subs {
		wg.Add(1)
		go func(channelID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			playlistID, err := ChannelUploadsPlaylistID(client, channelID)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return // one channel failing (e.g. it has no uploads) shouldn't sink the whole feed
			}
			videos, err := PlaylistVideosLimited(client, playlistID, perChannelLimit)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			all = append(all, videos...)
			mu.Unlock()
		}(sub.ChannelID)
	}
	wg.Wait()

	if len(all) == 0 && firstErr != nil {
		return nil, firstErr // every channel failed — surface the error
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].PublishedAt.After(all[j].PublishedAt)
	})
	return all, nil
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
		for _, it := range page.Items {
			if it.Snippet.ResourceID.VideoID == "" {
				continue // deleted/private video still listed as a stub entry
			}
			stubs = append(stubs, playlistItemStub{
				videoID:     it.Snippet.ResourceID.VideoID,
				title:       it.Snippet.Title,
				channel:     it.Snippet.VideoOwnerChannel,
				thumb:       it.Snippet.Thumbnails.Medium.URL,
				publishedAt: it.Snippet.PublishedAt,
			})
			if limit > 0 && len(stubs) >= limit {
				break
			}
		}
		if page.NextPageToken == "" || (limit > 0 && len(stubs) >= limit) {
			break
		}
		pageToken = page.NextPageToken
	}

	ids := make([]string, len(stubs))
	for i, s := range stubs {
		ids[i] = s.videoID
	}
	durations, views, err := videoDetails(client, ids)
	if err != nil {
		// Non-fatal — show the list without duration/views rather than
		// failing the whole playlist load over a secondary call.
		durations, views = map[string]float64{}, map[string]int64{}
	}

	out := make([]ytdlp.Video, 0, len(stubs))
	for _, s := range stubs {
		pub, _ := time.Parse(time.RFC3339, s.publishedAt)
		out = append(out, ytdlp.Video{
			ID:          s.videoID,
			Title:       s.title,
			Channel:     s.channel,
			Thumbnail:   s.thumb,
			URL:         "https://www.youtube.com/watch?v=" + s.videoID,
			Duration:    durations[s.videoID],
			ViewCount:   views[s.videoID],
			PublishedAt: pub,
		})
	}
	return out, nil
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
