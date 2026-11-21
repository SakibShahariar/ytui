// Package update checks GitHub releases for newer ytui versions. It never
// downloads or installs anything.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultEndpoint = "https://api.github.com/repos/SakibShahariar/ytui/releases/latest"
const DefaultInterval = 24 * time.Hour

// ErrNoReleases means the repository has not published a release yet.
var ErrNoReleases = errors.New("no releases published yet")

// Release is the subset of the GitHub release response ytui displays.
type Release struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
}

// Client is the HTTP surface used by Check. Tests supply a mock.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

// Check fetches the latest release from endpoint.
func Check(ctx context.Context, client Client, endpoint string) (Release, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ytui-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoReleases
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update check failed with HTTP %d", resp.StatusCode)
	}
	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("invalid release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return Release{}, fmt.Errorf("release response has no tag name")
	}
	return release, nil
}

func versionParts(version string) []int {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if index := strings.IndexAny(version, "-+"); index >= 0 {
		version = version[:index]
	}
	fields := strings.Split(version, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil {
			return nil
		}
		parts = append(parts, number)
	}
	return parts
}

// NewerThan reports whether latest is a newer release than current. Build
// metadata such as pseudo-version timestamps is ignored.
func NewerThan(current, latest string) bool {
	currentParts := versionParts(current)
	latestParts := versionParts(latest)
	if currentParts == nil {
		return true // dev or unparsable build: treat a tagged release as newer
	}
	if latestParts == nil {
		return false
	}
	for i := 0; i < len(currentParts) || i < len(latestParts); i++ {
		var a, b int
		if i < len(currentParts) {
			a = currentParts[i]
		}
		if i < len(latestParts) {
			b = latestParts[i]
		}
		if a != b {
			return b > a
		}
	}
	return false
}

// CachePath is the local file recording the last automatic check.
func CachePath() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "ytui", "update-check.json")
}

// ShouldCheck respects the configured interval for automatic checks.
func ShouldCheck(path string, interval time.Duration, now time.Time) bool {
	if interval <= 0 {
		interval = DefaultInterval
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var stamp struct {
		LastCheck time.Time `json:"last_check"`
	}
	if err := json.Unmarshal(data, &stamp); err != nil || stamp.LastCheck.IsZero() {
		return true
	}
	return now.Sub(stamp.LastCheck) >= interval
}

// RecordCheck stores the time of a completed automatic check.
func RecordCheck(path string, at time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		LastCheck time.Time `json:"last_check"`
	}{LastCheck: at})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-check-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
