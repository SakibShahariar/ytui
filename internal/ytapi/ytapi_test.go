package ytapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type apiRoundTripper func(*http.Request) (*http.Response, error)

func (f apiRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func apiResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(body)),
		Request: request,
	}
}

func TestHomeFeedBatchesVideoDetailsAcrossSubscriptions(t *testing.T) {
	const subscriptionCount = 100
	var mu sync.Mutex
	detailCalls := 0
	playlistCalls := 0
	client := &http.Client{Transport: apiRoundTripper(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		switch request.URL.Path {
		case "/youtube/v3/channels":
			channelID := query.Get("id")
			return apiResponse(request, http.StatusOK, fmt.Sprintf(`{"items":[{"contentDetails":{"relatedPlaylists":{"uploads":"uploads-%s"}}}]}`, channelID)), nil
		case "/youtube/v3/playlistItems":
			mu.Lock()
			playlistCalls++
			mu.Unlock()
			playlistID := query.Get("playlistId")
			channelID := strings.TrimPrefix(playlistID, "uploads-")
			body := fmt.Sprintf(`{"items":[{"snippet":{"title":"Video %s","videoOwnerChannelTitle":"Channel %s","publishedAt":"2026-10-31T12:00:00Z","thumbnails":{"medium":{"url":"https://example.invalid/thumb.jpg"}},"resourceId":{"videoId":"video-%s"}}}]}`, channelID, channelID, channelID)
			return apiResponse(request, http.StatusOK, body), nil
		case "/youtube/v3/videos":
			mu.Lock()
			detailCalls++
			mu.Unlock()
			ids := strings.Split(query.Get("id"), ",")
			items := make([]string, 0, len(ids))
			for _, id := range ids {
				items = append(items, fmt.Sprintf(`{"id":%q,"contentDetails":{"duration":"PT2M"},"statistics":{"viewCount":"123"}}`, id))
			}
			return apiResponse(request, http.StatusOK, `{"items":[`+strings.Join(items, ",")+`]}`), nil
		default:
			return apiResponse(request, http.StatusNotFound, `{"error":{"message":"unexpected endpoint"}}`), nil
		}
	})}

	subs := make([]Subscription, subscriptionCount)
	for i := range subs {
		subs[i] = Subscription{ChannelID: fmt.Sprintf("channel-home-batch-%d", i)}
	}
	videos, failed, err := HomeFeed(client, subs, 5)
	if err != nil { t.Fatalf("HomeFeed() error = %v", err) }
	if failed != 0 { t.Fatalf("HomeFeed() failed channels = %d, want 0", failed) }
	if len(videos) != subscriptionCount { t.Fatalf("HomeFeed() returned %d videos, want %d", len(videos), subscriptionCount) }
	mu.Lock()
	defer mu.Unlock()
	if playlistCalls != subscriptionCount { t.Fatalf("playlistItems calls = %d, want %d", playlistCalls, subscriptionCount) }
	if detailCalls != 2 { t.Fatalf("videos.list calls = %d, want two 50-ID batches", detailCalls) }
	for _, video := range videos {
		if video.Duration != 120 || video.ViewCount != 123 {
			t.Errorf("video %q details = duration %v, views %d; want 120 and 123", video.ID, video.Duration, video.ViewCount)
		}
	}
}

func TestVideoDetailsPartialRetainsSuccessfulBatches(t *testing.T) {
	ids := make([]string, 51)
	for i := range ids { ids[i] = fmt.Sprintf("video-%02d", i) }
	var calls int
	client := &http.Client{Transport: apiRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return apiResponse(request, http.StatusServiceUnavailable, `{"error":{"message":"temporary failure"}}`), nil
		}
		return apiResponse(request, http.StatusOK, `{"items":[{"id":"video-50","contentDetails":{"duration":"PT1M5S"},"statistics":{"viewCount":"42"}}]}`), nil
	})}

	durations, views, err := videoDetailsPartial(client, ids)
	if err == nil { t.Fatal("videoDetailsPartial() error = nil, want first-batch error") }
	if calls != 2 { t.Fatalf("videos.list calls = %d, want 2", calls) }
	if durations["video-50"] != 65 || views["video-50"] != 42 {
		t.Fatalf("successful second batch details = (%v, %d), want (65, 42)", durations["video-50"], views["video-50"])
	}
	if _, ok := durations["video-00"]; ok { t.Fatal("failed first batch unexpectedly populated duration") }
}

func TestUniquePlaylistVideoIDsPreservesFirstOccurrence(t *testing.T) {
	stubs := []playlistItemStub{{videoID: "one"}, {videoID: "two"}, {videoID: "one"}, {videoID: "three"}}
	got := uniquePlaylistVideoIDs(stubs)
	if strings.Join(got, ",") != "one,two,three" {
		t.Fatalf("uniquePlaylistVideoIDs() = %v, want [one two three]", got)
	}
}
