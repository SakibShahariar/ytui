// Package sharedqueue implements the optional inbox shared with the Ytrawl
// GNOME extension. It is deliberately independent of either UI.
package sharedqueue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxQueueBytes = 1 << 20

// Item is the version-1 interchange format written by Ytrawl and consumed by ytui.
type Item struct {
	ID string `json:"id"`
	URL string `json:"url"`
	Title string `json:"title,omitempty"`
	Mode string `json:"mode,omitempty"`
	Quality string `json:"quality,omitempty"`
	AudioFormat string `json:"audio_format,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type queueFile struct {
	Version int `json:"version"`
	Items []Item `json:"items"`
}

// Path returns the standard XDG state location. The override is useful for
// tests and for users who intentionally share a queue at a custom path.
func Path() (string, error) {
	if override := strings.TrimSpace(os.Getenv("YTUI_SHARED_QUEUE_PATH")); override != "" {
		if !filepath.IsAbs(override) { return "", fmt.Errorf("YTUI_SHARED_QUEUE_PATH must be absolute") }
		return filepath.Clean(override), nil
	}
	base := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil { return "", err }
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) { return "", fmt.Errorf("state directory must be absolute") }
	return filepath.Join(base, "ytui", "shared-queue.json"), nil
}

func isNumericHost(host string) bool {
	if host == "" { return false }
	for _, label := range strings.Split(host, ".") {
		if label == "" { return false }
		if strings.HasPrefix(strings.ToLower(label), "0x") {
			if len(label) <= 2 { return false }
			for _, r := range label[2:] {
				if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) { return false }
			}
			continue
		}
		for _, r := range label {
			if r < '0' || r > '9' { return false }
		}
	}
	return true
}

func validateItem(item Item) error {
	if item.ID == "" || len(item.ID) > 128 { return fmt.Errorf("shared queue item has an invalid ID") }
	for _, r := range item.ID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return fmt.Errorf("shared queue item ID contains invalid characters")
		}
	}
	if len(item.URL) == 0 || len(item.URL) > 4096 { return fmt.Errorf("shared queue item has an invalid URL length") }
	u, err := url.ParseRequestURI(strings.TrimSpace(item.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("shared queue item URL must be an absolute http(s) URL without embedded credentials")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".lan") || strings.HasSuffix(host, ".home.arpa") {
		return fmt.Errorf("shared queue URL must use a public host")
	}
	ipHost := host
	if zone := strings.IndexByte(ipHost, '%'); zone >= 0 { ipHost = ipHost[:zone] }
	if ip := net.ParseIP(ipHost); ip != nil &&
		(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return fmt.Errorf("shared queue URL must not target a private or local IP address")
	} else if ip == nil && isNumericHost(ipHost) {
		return fmt.Errorf("shared queue URL must not use a numeric host")
	}
	if item.URL != strings.TrimSpace(item.URL) { return fmt.Errorf("shared queue URL contains surrounding whitespace") }
	if strings.ContainsAny(item.Title+item.Quality+item.AudioFormat, "\x00\r\n\x1b") { return fmt.Errorf("shared queue metadata contains terminal control characters") }
	if item.Mode != "" && item.Mode != "video" && item.Mode != "audio" { return fmt.Errorf("shared queue item has unsupported mode %q", item.Mode) }
	if len(item.Title) > 512 || len(item.Quality) > 128 || len(item.AudioFormat) > 32 { return fmt.Errorf("shared queue item metadata is too long") }
	return nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil { return err }
	// The default ytui state directory is owned by this feature and should
	// remain private. A custom YTUI_SHARED_QUEUE_PATH may deliberately point
	// into an existing directory (including a shared temp directory); never
	// silently change permissions on that user-selected parent directory.
	if strings.TrimSpace(os.Getenv("YTUI_SHARED_QUEUE_PATH")) == "" {
		if err := os.Chmod(dir, 0o700); err != nil { return err }
	}
	return nil
}

func withLock(path string, fn func() error) error {
	if err := ensureDir(path); err != nil { return fmt.Errorf("prepare shared queue directory: %w", err) }
	lockPath := path + ".lock"
	var lock *os.File
	deadline := time.Now().Add(2 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			lock = file
			break
		}
		if !os.IsExist(err) { return fmt.Errorf("create shared queue lock: %w", err) }
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) { return fmt.Errorf("shared queue is locked by another process") }
		time.Sleep(25 * time.Millisecond)
	}
	_, _ = fmt.Fprintf(lock, "%d %d\n", os.Getpid(), time.Now().Unix())
	_ = lock.Close()
	defer os.Remove(lockPath)
	return fn()
}

func readUnlocked(path string) (queueFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) { return queueFile{Version: 1, Items: []Item{}}, nil }
	if err != nil { return queueFile{}, err }
	if len(data) > maxQueueBytes { return queueFile{}, fmt.Errorf("shared queue file exceeds %d bytes", maxQueueBytes) }
	var q queueFile
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(data), maxQueueBytes))
	if err := decoder.Decode(&q); err != nil { return queueFile{}, fmt.Errorf("parse shared queue file: %w", err) }
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF { return queueFile{}, fmt.Errorf("shared queue file contains trailing JSON data") }
	if q.Version != 1 { return queueFile{}, fmt.Errorf("unsupported shared queue version %d", q.Version) }
	if q.Items == nil { q.Items = []Item{} }
	seenIDs := make(map[string]struct{}, len(q.Items))
	for i, item := range q.Items {
		if err := validateItem(item); err != nil { return queueFile{}, fmt.Errorf("shared queue item %d: %w", i+1, err) }
		if _, exists := seenIDs[item.ID]; exists { return queueFile{}, fmt.Errorf("shared queue contains duplicate item ID %q", item.ID) }
		seenIDs[item.ID] = struct{}{}
	}
	return q, nil
}

func writeUnlocked(path string, q queueFile) error {
	if err := ensureDir(path); err != nil { return err }
	if q.Version == 0 { q.Version = 1 }
	if q.Items == nil { q.Items = []Item{} }
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil { return err }
	if len(data) > maxQueueBytes { return fmt.Errorf("shared queue would exceed %d bytes", maxQueueBytes) }
	tmp, err := os.CreateTemp(filepath.Dir(path), ".shared-queue-*.tmp")
	if err != nil { return err }
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil { tmp.Close(); return err }
	if _, err := tmp.Write(data); err != nil { tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	return os.Rename(tmpPath, path)
}

// Append adds an item once by ID. It never starts a download itself.
func Append(item Item) error {
	if err := validateItem(item); err != nil { return err }
	path, err := Path()
	if err != nil { return err }
	return withLock(path, func() error {
		q, err := readUnlocked(path)
		if err != nil { return err }
		for _, existing := range q.Items { if existing.ID == item.ID { return nil } }
		q.Items = append(q.Items, item)
		return writeUnlocked(path, q)
	})
}

// Read returns a snapshot of pending inbox items. Missing files mean an empty queue.
func Read() ([]Item, error) {
	path, err := Path()
	if err != nil { return nil, err }
	var items []Item
	err = withLock(path, func() error {
		q, err := readUnlocked(path)
		if err != nil { return err }
		items = append([]Item(nil), q.Items...)
		return nil
	})
	return items, err
}

// Ack removes only the IDs ytui successfully accepted. Items added by Ytrawl
// after Read remain untouched, so a concurrent producer cannot lose new work.
func Ack(ids []string) error {
	if len(ids) == 0 { return nil }
	path, err := Path()
	if err != nil { return err }
	remove := make(map[string]bool, len(ids))
	for _, id := range ids { remove[id] = true }
	return withLock(path, func() error {
		q, err := readUnlocked(path)
		if err != nil { return err }
		kept := make([]Item, 0, len(q.Items))
		for _, item := range q.Items { if !remove[item.ID] { kept = append(kept, item) } }
		q.Items = kept
		return writeUnlocked(path, q)
	})
}
