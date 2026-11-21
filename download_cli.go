package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

// exitError carries a script-friendly status code out of a subcommand.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

// getVideoForDownload is replaceable in tests so the CLI can be exercised
// without network access.
var getVideoForDownload = ytdlp.GetVideo

// downloadCLI polls a snapshot of the queue because the manager intentionally
// exposes copies rather than live download pointers.
func runDownloadCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "", "yt-dlp format selector (default: best video+audio)")
	fs.StringVar(format, "f", "", "yt-dlp format selector (shorthand)")
	audio := fs.Bool("audio", false, "download audio only")
	fs.BoolVar(audio, "a", false, "download audio only (shorthand)")
	audioFormatFlag := fs.String("audio-format", "mp3", "audio-only format: mp3 or opus")
	subs := fs.Bool("subs", false, "download subtitle sidecars when available")
	fs.BoolVar(subs, "s", false, "download subtitle sidecars (shorthand)")
	subLang := fs.String("sub-lang", "en.*,en", "subtitle language selector")
	rate := fs.String("rate", "", "rate limit such as 500K or 2M (overrides config)")
	sha256 := fs.String("sha256", "", "expected SHA-256 hex digest of the completed file")
	sha1 := fs.String("sha1", "", "expected SHA-1 hex digest of the completed file")
	md5 := fs.String("md5", "", "expected MD5 hex digest of the completed file")
	queueOnly := fs.Bool("queue", false, "enqueue and exit without waiting")
	quiet := fs.Bool("quiet", false, "suppress progress output")
	fs.BoolVar(quiet, "q", false, "suppress progress output (shorthand)")
	if err := fs.Parse(args); err != nil {
		return &exitError{code: 2, err: err}
	}
	if fs.NArg() != 1 {
		return &exitError{code: 2, err: fmt.Errorf("usage: ytui download [flags] URL")}
	}
	videoURL, err := validateVideoURL(fs.Arg(0))
	if err != nil {
		return &exitError{code: 2, err: fmt.Errorf("invalid video URL: %w", err)}
	}
	audioFormat := strings.ToLower(strings.TrimSpace(*audioFormatFlag))
	if *audio && audioFormat != "mp3" && audioFormat != "opus" {
		return &exitError{code: 2, err: fmt.Errorf("--audio-format must be mp3 or opus")}
	}

	checksumFlags := []struct {
		algorithm string
		value     string
	}{
		{downloader.ChecksumSHA256, *sha256},
		{downloader.ChecksumSHA1, *sha1},
		{downloader.ChecksumMD5, *md5},
	}
	checksumAlgorithm, checksum := "", ""
	for _, flag := range checksumFlags {
		if strings.TrimSpace(flag.value) == "" {
			continue
		}
		if checksum != "" {
			return &exitError{code: 2, err: fmt.Errorf("use only one of --sha256, --sha1, or --md5")}
		}
		checksum = flag.value
		checksumAlgorithm = flag.algorithm
	}
	if checksum != "" {
		normalized, err := downloader.ParseChecksum(checksumAlgorithm, checksum)
		if err != nil {
			return &exitError{code: 2, err: err}
		}
		checksum = normalized
	}

	cfg := config.Load()
	if value := strings.TrimSpace(*rate); value != "" {
		canonical, err := config.NormalizeRateLimit(value)
		if err != nil {
			return &exitError{code: 2, err: err}
		}
		cfg.DownloadRateLimit = canonical
	}
	dir := cfg.DownloadDir
	if override := os.Getenv("YTUI_DOWNLOAD_DIR"); override != "" {
		dir = override
	}
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Downloads", "ytui")
	}

	manager, err := downloader.NewWithConcurrency(dir, cfg.MaxConcurrentDownloads)
	if err != nil {
		return &exitError{code: 1, err: fmt.Errorf("create download manager: %w", err)}
	}
	defer manager.Shutdown()

	video, err := getVideoForDownload(videoURL)
	if err != nil {
		return &exitError{code: 1, err: fmt.Errorf("resolve video: %w", err)}
	}
	opts := downloader.Options{
		Format:             *format,
		ExtractAudio:       *audio,
		WriteSubs:          *subs,
		SubtitleLang:       *subLang,
		CookiesFromBrowser: cfg.DefaultCookiesBrowser,
		RateLimit:          cfg.DownloadRateLimit,
		ChecksumAlgorithm:  checksumAlgorithm,
		ExpectedChecksum:   checksum,
		RetryAttempts:      cfg.DownloadRetryAttempts,
		RetryBackoff:       time.Duration(cfg.DownloadRetryBackoffSeconds) * time.Second,
	}
	if *audio {
		opts.AudioFormat = audioFormat
		opts.QualityLabel = "Audio only (" + audioFormat + ")"
	} else if *format == "" {
		opts.QualityLabel = "Best quality (video+audio)"
	} else {
		opts.QualityLabel = *format
	}
	queued := manager.Enqueue(video.ID, video.Title, video.Channel, video.URL, opts)
	if *queueOnly {
		fmt.Fprintf(stdout, "queued: %s\n", video.Title)
		return nil
	}
	if !*quiet {
		fmt.Fprintf(stdout, "downloading: %s\n", video.Title)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	lastProgress := -1.0
	for range ticker.C {
		var current *downloader.Download
		for _, d := range manager.List() {
			if d.VideoID == queued.VideoID {
				copy := d
				current = &copy
				break
			}
		}
		if current == nil {
			return &exitError{code: 1, err: fmt.Errorf("download disappeared from the queue")}
		}
		switch current.Status {
		case downloader.StatusDone:
			if !*quiet {
				fmt.Fprintf(stdout, "done: %s\n", current.FilePath)
			}
			return nil
		case downloader.StatusError, downloader.StatusResumable, downloader.StatusCanceled:
			msg := "download failed"
			if current.Err != nil {
				msg = current.Err.Error()
			}
			return &exitError{code: 1, err: fmt.Errorf("%s", msg)}
		}
		if !*quiet && current.Status == downloader.StatusDownloading && current.Progress != lastProgress {
			lastProgress = current.Progress
			fmt.Fprintf(stdout, "%.1f%% %s\n", current.Progress, current.Phase)
		}
	}
	return nil
}
