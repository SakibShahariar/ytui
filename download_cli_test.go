package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func setupDownloadCLI(t *testing.T, script string) string {
	t.Helper()
	t.Setenv("YTUI_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	t.Setenv("YTUI_DOWNLOAD_DIR", dir)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	original := getVideoForDownload
	getVideoForDownload = func(rawURL string) (ytdlp.Video, error) {
		return ytdlp.Video{ID: "cli-test", Title: "CLI Test", Channel: "Test Channel", URL: rawURL}, nil
	}
	t.Cleanup(func() { getVideoForDownload = original })
	return dir
}

func TestDownloadCommandQueuesWithoutTUI(t *testing.T) {
	dir := setupDownloadCLI(t, "#!/bin/sh\nexit 1\n")
	var out, errOut strings.Builder
	err := runDownloadCommand([]string{"--audio", "--rate", "500K", "--subs", "--queue",
		"https://www.youtube.com/watch?v=cli-test"}, &out, &errOut)
	if err != nil {
		t.Fatalf("runDownloadCommand() error = %v", err)
	}
	if !strings.Contains(out.String(), "queued: CLI Test") {
		t.Fatalf("output = %q, want queued confirmation", out.String())
	}
	queuePath := filepath.Join(dir, ".ytui-queue.json")
	if _, statErr := os.Stat(queuePath); statErr != nil {
		t.Fatalf("queue file missing: %v", statErr)
	}
	manager, managerErr := downloader.New(dir)
	if managerErr != nil {
		t.Fatal(managerErr)
	}
	defer manager.Shutdown()
	entries := manager.List()
	if len(entries) != 1 {
		t.Fatalf("restored queue has %d entries, want 1", len(entries))
	}
	opts := entries[0].Opts
	if !opts.ExtractAudio || opts.AudioFormat != "mp3" || opts.RateLimit != "500K" || !opts.WriteSubs {
		t.Fatalf("restored options = %#v, want audio/mp3/500K/subs", opts)
	}
}

func TestDownloadCommandExitCodes(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		setupDownloadCLI(t, "#!/bin/sh\nexit 0\n")
		var out, errOut strings.Builder
		err := runDownloadCommand([]string{"not-a-url"}, &out, &errOut)
		var exitErr *exitError
		if !errors.As(err, &exitErr) || exitErr.code != 2 {
			t.Fatalf("error = %v, want exit code 2", err)
		}
	})
	t.Run("invalid audio format", func(t *testing.T) {
		setupDownloadCLI(t, "#!/bin/sh\nexit 0\n")
		var out, errOut strings.Builder
		err := runDownloadCommand([]string{"--audio", "--audio-format", "flac", "https://example.com/v"}, &out, &errOut)
		var exitErr *exitError
		if !errors.As(err, &exitErr) || exitErr.code != 2 {
			t.Fatalf("error = %v, want exit code 2", err)
		}
	})
	t.Run("invalid rate", func(t *testing.T) {
		setupDownloadCLI(t, "#!/bin/sh\nexit 0\n")
		var out, errOut strings.Builder
		err := runDownloadCommand([]string{"--rate", "1T", "https://example.com/v"}, &out, &errOut)
		var exitErr *exitError
		if !errors.As(err, &exitErr) || exitErr.code != 2 {
			t.Fatalf("error = %v, want exit code 2", err)
		}
	})
	t.Run("successful download", func(t *testing.T) {
		dir := setupDownloadCLI(t, "#!/bin/sh\nprintf '[download] Destination: %s\\n' \"$YTUI_TEST_FINAL\"\n: > \"$YTUI_TEST_FINAL\"\n")
		final := filepath.Join(dir, "cli-output.mp4")
		t.Setenv("YTUI_TEST_FINAL", final)
		var out, errOut strings.Builder
		err := runDownloadCommand([]string{"https://example.com/v"}, &out, &errOut)
		if err != nil {
			t.Fatalf("runDownloadCommand() error = %v", err)
		}
		if !strings.Contains(out.String(), "done: "+final) {
			t.Fatalf("output = %q, want final path", out.String())
		}
	})
	t.Run("failed download", func(t *testing.T) {
		setupDownloadCLI(t, "#!/bin/sh\necho 'ERROR: failure' >&2\nexit 1\n")
		var out, errOut strings.Builder
		err := runDownloadCommand([]string{"https://example.com/v"}, &out, &errOut)
		var exitErr *exitError
		if !errors.As(err, &exitErr) || exitErr.code != 1 {
			t.Fatalf("error = %v, want exit code 1", err)
		}
	})
}
