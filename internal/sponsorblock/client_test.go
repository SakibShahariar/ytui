package sponsorblock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUsesHashPrefixAndFiltersMatchingSegments(t *testing.T) {
	videoID := "private-video-id"
	hashBytes := sha256.Sum256([]byte(videoID))
	fullHash := hex.EncodeToString(hashBytes[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/skipSegments/"+fullHash[:4] { t.Errorf("request path = %q; want hash prefix only", r.URL.Path) }
		if r.URL.Query().Get("videoID") != "" { t.Error("request exposed raw video ID") }
		if got := r.URL.Query().Get("categories"); got != "[\"selfpromo\",\"sponsor\"]" { t.Errorf("categories = %q", got) }
		fmt.Fprintf(w, `[
			{"videoID":"other","hash":"deadbeef","segments":[{"segment":[1,2],"category":"sponsor"}]},
			{"videoID":"ignored","hash":"%s","segments":[
				{"segment":[20,30],"category":"selfpromo","actionType":"skip","UUID":"b"},
				{"segment":[0,8],"category":"sponsor","actionType":"skip","UUID":"a"},
				{"segment":[4,4],"category":"sponsor","actionType":"skip"},
				{"segment":[5,9],"category":"sponsor","actionType":"mute"}
			]}
		]`, fullHash)
	}))
	defer server.Close()
	segments, err := fetchWith(context.Background(), server.Client(), server.URL, videoID, []string{"selfpromo", "sponsor", "sponsor"})
	if err != nil { t.Fatalf("fetchWith() error = %v", err) }
	if len(segments) != 2 { t.Fatalf("segments = %#v, want two valid skip segments", segments) }
	if segments[0].Start != 0 || segments[0].End != 8 || segments[0].Category != "sponsor" { t.Fatalf("first segment = %#v", segments[0]) }
	if segments[1].Start != 20 || segments[1].End != 30 || segments[1].Category != "selfpromo" { t.Fatalf("second segment = %#v", segments[1]) }
}

func TestFetchNotFoundIsAnEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	segments, err := fetchWith(context.Background(), server.Client(), server.URL, "video", []string{"sponsor"})
	if err != nil || len(segments) != 0 { t.Fatalf("fetchWith() = %#v, %v; want empty result without error", segments, err) }
}

func TestFetchRejectsInvalidInputAndHandlesEmptyCategories(t *testing.T) {
	if _, err := fetchWith(context.Background(), http.DefaultClient, "https://example.invalid", "", []string{"sponsor"}); err == nil { t.Fatal("empty video ID unexpectedly succeeded") }
	segments, err := fetchWith(context.Background(), http.DefaultClient, "://bad", "video", nil)
	if err != nil || len(segments) != 0 { t.Fatalf("empty categories = %#v, %v; want no-op", segments, err) }
}
