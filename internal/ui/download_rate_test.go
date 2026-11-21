package ui

import (
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/downloader"
)

func TestDownloadOptionsSummaryIncludesRateLimit(t *testing.T) {
	got := downloadOptionsSummary(downloader.Options{
		QualityLabel: "Best quality (video+audio)", RateLimit: "2M",
	})
	want := "Best quality (video+audio) · limit: 2M/s"
	if got != want {
		t.Fatalf("downloadOptionsSummary() = %q, want %q", got, want)
	}
}

func TestEffectiveDownloadRateLimit(t *testing.T) {
	m := Model{cfg: config.Config{DownloadRateLimit: "1M"}}
	if got := m.effectiveDownloadRateLimit(); got != "1M" {
		t.Fatalf("default rate = %q, want config value 1M", got)
	}
	m.downloadPickerRate = 3 // "2M"
	if got := m.effectiveDownloadRateLimit(); got != "2M" {
		t.Fatalf("picker rate = %q, want 2M", got)
	}
	m.downloadPickerRate = 0
	m.cfg.DownloadRateLimit = ""
	if got := m.effectiveDownloadRateLimit(); got != "" {
		t.Fatalf("unlimited rate = %q, want empty", got)
	}
}

func TestDownloadPickerShowsRateLimitRow(t *testing.T) {
	if downloadPickerRows() != len(downloadQualities)+5 {
		t.Fatalf("downloadPickerRows() = %d, want %d", downloadPickerRows(), len(downloadQualities)+5)
	}
	m := Model{
		cfg:                   config.Config{DownloadRateLimit: "1M"},
		downloadPickerQuality: 0,
		downloadPickerCursor:  len(downloadQualities) + 3,
	}
	view := m.downloadPickerContent(80)
	if !strings.Contains(view, "Rate limit:") || !strings.Contains(view, "1M/s") {
		t.Fatalf("picker content missing active rate limit:\n%s", view)
	}
	startRow := len(downloadQualities) + 4
	if !strings.Contains(view, "▶ Start Download") {
		t.Fatalf("picker content missing Start Download row:\n%s", view)
	}
	if m.downloadPickerCursor != startRow-1 {
		t.Fatalf("cursor = %d, want rate row %d", m.downloadPickerCursor, startRow-1)
	}
}
