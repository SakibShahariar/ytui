package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestDownloadPresetsListAndApply(t *testing.T) {
	m := Model{cfg: config.Config{DownloadPresets: []config.DownloadPreset{
		{Name: "Podcast", Quality: "Audio only (mp3)", RateLimit: "500K"},
	}}}
	presets := m.downloadPresets()
	if len(presets) != len(builtinDownloadPresets)+1 {
		t.Fatalf("presets = %d, want %d", len(presets), len(builtinDownloadPresets)+1)
	}
	if presets[len(presets)-1].Name != "Podcast" {
		t.Fatalf("last preset = %q, want Podcast", presets[len(presets)-1].Name)
	}

	// Built-in preset applies its quality and extras.
	m.applyDownloadPreset(5) // Video + subtitles
	if downloadQualities[m.downloadPickerQuality].Label != "Best quality (video+audio)" || !m.downloadPickerSubs || m.downloadPickerThumb {
		t.Fatalf("applied preset state = quality %d, subs %v, thumb %v", m.downloadPickerQuality, m.downloadPickerSubs, m.downloadPickerThumb)
	}
	// Manual overrides after a preset remain effective.
	m.downloadPickerSubs = false
	if m.downloadPickerSubs {
		t.Fatal("manual override did not stick")
	}

	// Custom preset with a known rate choice.
	m.applyDownloadPreset(len(presets) - 1)
	if downloadQualities[m.downloadPickerQuality].Label != "Audio only (mp3)" {
		t.Fatalf("custom preset quality = %q", downloadQualities[m.downloadPickerQuality].Label)
	}
	if got := m.effectiveDownloadRateLimit(); got != "500K" {
		t.Fatalf("custom preset rate = %q, want 500K", got)
	}

	// Custom rate outside the standard choices is preserved.
	m.cfg.DownloadPresets = []config.DownloadPreset{{Name: "Slow", Quality: "720p", RateLimit: "123K"}}
	presets = m.downloadPresets()
	m.applyDownloadPreset(len(presets) - 1)
	if got := m.effectiveDownloadRateLimit(); got != "123K" {
		t.Fatalf("custom rate = %q, want 123K", got)
	}
}

func TestPresetAvailableInCommandPalette(t *testing.T) {
	m := Model{}
	m.commandPaletteInput.SetValue("preset:")
	commands := m.filteredPaletteCommands()
	if len(commands) != len(builtinDownloadPresets) {
		t.Fatalf("palette preset commands = %d, want %d", len(commands), len(builtinDownloadPresets))
	}
	m.list = list.New([]list.Item{videoItem{v: ytdlp.Video{ID: "abc", Title: "Video", URL: "https://example.com/v"}}}, list.NewDefaultDelegate(), 60, 10)
	updated, _ := m.executePaletteCommand("preset:5") // Video + subtitles
	result := updated.(Model)
	if !result.downloadPicker || !result.downloadPickerSubs || result.downloadPickerQuality != 0 {
		t.Fatalf("palette preset state = picker %v, subs %v, quality %d", result.downloadPicker, result.downloadPickerSubs, result.downloadPickerQuality)
	}
}

func TestDownloadPickerShowsPresetRow(t *testing.T) {
	if downloadPickerRows() != len(downloadQualities)+6 {
		t.Fatalf("downloadPickerRows() = %d", downloadPickerRows())
	}
	m := Model{downloadPickerCursor: len(downloadQualities) + 4}
	view := m.downloadPickerContent(80)
	if !strings.Contains(view, "Preset:") || !strings.Contains(view, "▶ Start Download") {
		t.Fatalf("picker missing preset row or Start Download:\n%s", view)
	}
}
