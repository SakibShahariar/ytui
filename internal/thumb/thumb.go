// Package thumb downloads video thumbnails and displays them using the
// Kitty terminal graphics protocol (via the `kitty +kitten icat` CLI,
// which ships with Kitty). This is Kitty-specific — most terminals
// (including GNOME Terminal) have no image protocol at all, so callers
// must check Supported() first and fall back to a text placeholder.
package thumb

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if _, err := exec.LookPath("kitty"); err != nil {
		return false
	}
	return false
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

// Download fetches a thumbnail by video ID, caching it on disk so repeat
// views (e.g. re-selecting a video, or watch-later/history entries) don't
// re-fetch. Returns the local file path.
func Download(url, videoID string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("no thumbnail URL")
	}
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, videoID+".jpg")

	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil // already cached
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

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	f.Close()
	return path, os.Rename(tmp, path)
}

// Show draws the image at path into the terminal cell region starting at
// (col, row) [0-indexed], sized (cols x rows) character cells. Requires
// Kitty (or compatible). This writes directly to the real terminal stdout,
// bypassing Bubbletea's render loop — that's unavoidable with the current
// state of terminal graphics protocols, and is the same technique used by
// TUI file managers like yazi/ranger for image previews.
func Show(path string, col, row, cols, rows int) error {
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
	_, err := os.Stdout.Write(stdout.Bytes())
	return err
}

// Clear removes any currently-displayed Kitty graphics image (all of them —
// the icat protocol doesn't give us a clean single-image handle from the
// CLI, so we clear-and-redraw on every selection change instead).
func Clear() error {
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
	_, err := os.Stdout.Write(stdout.Bytes())
	return err
}
