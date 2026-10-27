package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

const homeFeedCacheTTL = 10 * time.Minute

type homeFeedCache struct {
	CachedAt time.Time    `json:"cached_at"`
	Videos   []ytdlp.Video `json:"videos"`
}

func homeFeedCachePath() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheRoot, "ytui", "home-feed.json"), nil
}

func loadHomeFeedCache() ([]ytdlp.Video, time.Duration, error) {
	path, err := homeFeedCachePath()
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var cached homeFeedCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, 0, fmt.Errorf("decode Home feed cache: %w", err)
	}
	if cached.CachedAt.IsZero() || len(cached.Videos) == 0 {
		return nil, 0, fmt.Errorf("Home feed cache has no usable entries")
	}
	age := time.Since(cached.CachedAt)
	if age < 0 {
		age = 0
	}
	return cached.Videos, age, nil
}

func saveHomeFeedCache(videos []ytdlp.Video) error {
	if len(videos) == 0 {
		return fmt.Errorf("refusing to cache an empty Home feed")
	}
	path, err := homeFeedCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(homeFeedCache{CachedAt: time.Now(), Videos: videos})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".home-feed-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func homeFeedErrorMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	if strings.Contains(lower, "quota") || strings.Contains(lower, "403") || strings.Contains(lower, "forbidden") {
		return "YouTube API quota or authorization error: " + message
	}
	return message
}
