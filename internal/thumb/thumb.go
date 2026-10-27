// Package thumb downloads video thumbnails and displays them using the
// Kitty terminal graphics protocol (via the `kitty +kitten icat` CLI,
// which ships with Kitty). This is Kitty-specific — most terminals
// (including GNOME Terminal) have no image protocol at all, so callers
// must check Supported() first and fall back to a text placeholder.
package thumb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Supported reports whether the current terminal is Kitty (or a
// Kitty-graphics-compatible terminal advertising itself as such via
// $TERM / $KITTY_WINDOW_ID).
func Supported() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if strings.Contains(os.Getenv("TERM"), "kitty") {
		return true
	}
	// Some terminals (e.g. Ghostty, WezTerm in kitty-emulation mode) set this.
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return true
	}
	// The installed Kitty executable says nothing about which terminal is
	// currently hosting ytui. Only advertise previews when the terminal
	// identifies itself as supporting the Kitty graphics protocol.
	return false
}

const (
	DefaultMaxEntries = 250
	DefaultMaxAgeDays = 30
	staleTempAge = 24 * time.Hour
)

type CachePolicy struct {
	MaxEntries int
	MaxAgeDays int
}

type CacheStats struct {
	Directory string
	Entries   int
	Bytes     int64
}

type cacheEntry struct {
	path    string
	modTime time.Time
	size    int64
}

var (
	cacheMu sync.Mutex
	activePreviewPath string
)

func normalizedPolicy(policy CachePolicy) CachePolicy {
	if policy.MaxEntries <= 0 {
		policy.MaxEntries = DefaultMaxEntries
	}
	if policy.MaxAgeDays <= 0 {
		policy.MaxAgeDays = DefaultMaxAgeDays
	}
	return policy
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir() // ~/.cache on Linux
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ytui", "thumbs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Download fetches a thumbnail by video ID using the built-in cache policy.
func Download(url, videoID string) (string, error) {
	return DownloadWithPolicy(url, videoID, CachePolicy{})
}

// DownloadWithPolicy fetches a thumbnail and prunes the cache using the
// supplied limits. Zero-valued limits use the documented defaults.
func DownloadWithPolicy(url, videoID string, policy CachePolicy) (string, error) {
	if url == "" {
		return "", fmt.Errorf("no thumbnail URL")
	}
	policy = normalizedPolicy(policy)
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, videoID+".jpg")

	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		now := time.Now()
		_ = os.Chtimes(path, now, now) // treat reuse as recent use for LRU pruning
		_, _ = cleanupCache(policy, path)
		return path, nil
	}

	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("thumbnail download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("thumbnail download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp(dir, ".thumb-*.tmp")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	_, _ = cleanupCache(policy, path) // protect the current and requested previews during transition
	return path, nil
}

// CacheStats reports the current on-disk thumbnail count and size without
// reading image contents or touching authentication data.
func GetCacheStats() (CacheStats, error) {
	dir, err := cacheDir()
	if err != nil {
		return CacheStats{}, err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return CacheStats{}, err
	}
	stats := CacheStats{Directory: dir}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jpg") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return CacheStats{}, err
		}
		stats.Entries++
		stats.Bytes += info.Size()
	}
	return stats, nil
}

// CleanupCache removes expired thumbnails and then the least-recently-used
// entries above the configured entry limit. The currently displayed preview
// is protected until the terminal image is cleared or replaced. Old temporary
// files from interrupted writes are removed after 24 hours.
func CleanupCache(policy CachePolicy) (CacheStats, error) {
	return cleanupCache(policy, "")
}

func cleanupCache(policy CachePolicy, protectPath string) (CacheStats, error) {
	policy = normalizedPolicy(policy)
	dir, err := cacheDir()
	if err != nil {
		return CacheStats{}, err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()

	files, err := os.ReadDir(dir)
	if err != nil {
		return CacheStats{}, err
	}
	now := time.Now()
	maxAge := time.Duration(policy.MaxAgeDays) * 24 * time.Hour
	entries := make([]cacheEntry, 0, len(files))
	var cleanupErrs []error
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		path := filepath.Join(dir, file.Name())
		info, err := file.Info()
		if err != nil {
			cleanupErrs = append(cleanupErrs, err)
			continue
		}
		if strings.HasSuffix(file.Name(), ".tmp") {
			if now.Sub(info.ModTime()) > staleTempAge {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					cleanupErrs = append(cleanupErrs, err)
				}
			}
			continue
		}
		if !strings.HasSuffix(file.Name(), ".jpg") {
			continue
		}
		if path != activePreviewPath && path != protectPath && now.Sub(info.ModTime()) > maxAge {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				cleanupErrs = append(cleanupErrs, err)
				entries = append(entries, cacheEntry{path: path, modTime: info.ModTime(), size: info.Size()})
			}
			continue
		}
		entries = append(entries, cacheEntry{path: path, modTime: info.ModTime(), size: info.Size()})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].modTime.Before(entries[j].modTime) })
	for len(entries) > policy.MaxEntries {
		removeIndex := -1
		for i, entry := range entries {
			if entry.path != activePreviewPath && entry.path != protectPath {
				removeIndex = i
				break
			}
		}
		if removeIndex < 0 {
			break
		}
		if err := os.Remove(entries[removeIndex].path); err != nil && !os.IsNotExist(err) {
			cleanupErrs = append(cleanupErrs, err)
			// Avoid looping forever if this file cannot be removed.
			entries[removeIndex].modTime = time.Time{}
			entries[removeIndex].path = activePreviewPath
			continue
		}
		entries = append(entries[:removeIndex], entries[removeIndex+1:]...)
	}

	stats := CacheStats{Directory: dir, Entries: len(entries)}
	for _, entry := range entries {
		stats.Bytes += entry.size
	}
	return stats, errors.Join(cleanupErrs...)
}

// Downloaded thumbnails remain on disk until age or entry limits require
// cleanup. The active terminal preview is recorded while Show is successful.


// Show draws the image at path into the terminal cell region starting at
// (col, row) [0-indexed], sized (cols x rows) character cells. Requires
// Kitty (or compatible). This writes directly to the real terminal stdout,
// bypassing Bubbletea's render loop — that's unavoidable with the current
// state of terminal graphics protocols, and is the same technique used by
// TUI file managers like yazi/ranger for image previews.
func Show(path string, col, row, cols, rows int) error {
	return ShowWithPolicy(path, col, row, cols, rows, CachePolicy{})
}

// ShowWithPolicy marks the new preview active before enforcing limits, so
// pruning can remove the previous image without deleting the new one.
func ShowWithPolicy(path string, col, row, cols, rows int, policy CachePolicy) error {
	err := func() error {
		cacheMu.Lock()
		defer cacheMu.Unlock()
		cmd := exec.Command("kitty", "+kitten", "icat",
		"--transfer-mode=memory",
		"--stdin=no",
		"--align=left",
		"--scale-up",
		"--place", fmt.Sprintf("%dx%d@%dx%d", cols, rows, col, row),
		path,
	)
	// Buffer both streams instead of piping stdout live — icat sometimes
	// writes its actual error message to stdout, not stderr, and piping
	// it straight to the real terminal was silently swallowing it.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = strings.TrimSpace(stdout.String())
			}
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("icat: %s", msg)
		}
		// Success — forward the captured image bytes to the real terminal now.
		if _, err := os.Stdout.Write(stdout.Bytes()); err != nil {
			return err
		}
		activePreviewPath = path
		return nil
	}()
	if err != nil {
		return err
	}
	_, _ = CleanupCache(policy)
	return nil
}

// Clear removes any currently-displayed Kitty graphics image (all of them —
// the icat protocol doesn't give us a clean single-image handle from the
// CLI, so we clear-and-redraw on every selection change instead).
func Clear() error {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cmd := exec.Command("kitty", "+kitten", "icat", "--clear", "--transfer-mode=memory")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("icat clear: %s", msg)
	}
	if _, err := os.Stdout.Write(stdout.Bytes()); err != nil {
		return err
	}
	activePreviewPath = ""
	return nil
}
