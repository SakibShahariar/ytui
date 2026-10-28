// Package sponsorblock fetches public SponsorBlock segment metadata.
package sponsorblock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const apiBaseURL = "https://sponsor.ajay.app"

var Categories = []string{
	"sponsor", "selfpromo", "interaction", "intro", "outro", "preview",
	"music_offtopic", "filler", "exclusive_access", "hook", "poi_highlight", "chapter",
}

// SkipCategories are categories whose SponsorBlock action type can be skipped automatically.
var SkipCategories = []string{
	"sponsor", "selfpromo", "interaction", "intro", "outro", "preview",
	"music_offtopic", "filler", "exclusive_access", "hook",
}

// Segment is a validated time range returned by SponsorBlock.
type Segment struct {
	Start float64
	End float64
	Category string
	ActionType string
	UUID string
}

type segmentJSON struct {
	Segment []float64 `json:"segment"`
	UUID string `json:"UUID"`
	Category string `json:"category"`
	ActionType string `json:"actionType"`
}

type videoSegmentsJSON struct {
	VideoID string `json:"videoID"`
	Hash string `json:"hash"`
	Segments []segmentJSON `json:"segments"`
}

// Fetch requests segment data using the first four characters of the video's
// SHA-256 hash, never sending the raw video ID to SponsorBlock.
func Fetch(ctx context.Context, videoID string, categories []string) ([]Segment, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	return fetchWith(ctx, client, apiBaseURL, videoID, categories)
}

func fetchWith(ctx context.Context, client *http.Client, baseURL, videoID string, categories []string) ([]Segment, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" { return nil, fmt.Errorf("video ID is empty") }
	if len(categories) == 0 { return nil, nil }
	cleanCategories := make([]string, 0, len(categories))
	seenCategories := make(map[string]bool, len(categories))
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" || seenCategories[category] { continue }
		cleanCategories = append(cleanCategories, category)
		seenCategories[category] = true
	}
	if len(cleanCategories) == 0 { return nil, nil }
	sort.Strings(cleanCategories)
	hashBytes := sha256.Sum256([]byte(videoID))
	fullHash := hex.EncodeToString(hashBytes[:])
	prefix := fullHash[:4]
	categoryJSON, err := json.Marshal(cleanCategories)
	if err != nil { return nil, fmt.Errorf("encode SponsorBlock categories: %w", err) }
	actionTypes, _ := json.Marshal([]string{"skip", "poi", "chapter"})
	endpoint := strings.TrimRight(baseURL, "/") + "/api/skipSegments/" + prefix
	query := url.Values{}
	query.Set("categories", string(categoryJSON))
	query.Set("actionTypes", string(actionTypes))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil { return nil, fmt.Errorf("build SponsorBlock request: %w", err) }
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ytui")
	response, err := client.Do(request)
	if err != nil { return nil, fmt.Errorf("SponsorBlock request failed: %w", err) }
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound { return nil, nil }
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("SponsorBlock returned HTTP %d", response.StatusCode)
	}
	var results []videoSegmentsJSON
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&results); err != nil { return nil, fmt.Errorf("decode SponsorBlock response: %w", err) }
	var segments []Segment
	for _, result := range results {
		matches := result.Hash == fullHash
		if result.Hash == "" && result.VideoID != "" {
			hash := sha256.Sum256([]byte(result.VideoID))
			matches = hex.EncodeToString(hash[:]) == fullHash
		}
		if !matches { continue }
		for _, raw := range result.Segments {
			if len(raw.Segment) != 2 { continue }
			start, end := raw.Segment[0], raw.Segment[1]
			if start < 0 || end <= start || raw.Category == "" { continue }
			if raw.ActionType == "" { raw.ActionType = "skip" }
			if raw.ActionType != "skip" && raw.ActionType != "poi" && raw.ActionType != "chapter" { continue }
			segments = append(segments, Segment{Start: start, End: end, Category: raw.Category, ActionType: raw.ActionType, UUID: raw.UUID})
		}
	}
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].Start == segments[j].Start { return segments[i].End < segments[j].End }
		return segments[i].Start < segments[j].Start
	})
	return segments, nil
}
