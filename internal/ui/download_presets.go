package ui

import (
	"github.com/SakibShahariar/ytui/internal/config"
)

// builtinDownloadPresets cover the common download configurations without
// requiring any user configuration.
var builtinDownloadPresets = []config.DownloadPreset{
	{Name: "Best video", Quality: "Best quality (video+audio)"},
	{Name: "1080p", Quality: "1080p"},
	{Name: "720p", Quality: "720p"},
	{Name: "Audio (mp3)", Quality: "Audio only (mp3)"},
	{Name: "Audio (opus)", Quality: "Audio only (opus)"},
	{Name: "Video + subtitles", Quality: "Best quality (video+audio)", WriteSubs: true},
	{Name: "Video + thumbnail", Quality: "Best quality (video+audio)", EmbedThumbnail: true},
}

// downloadPresets returns built-in presets followed by user-defined ones.
func (m Model) downloadPresets() []config.DownloadPreset {
	presets := make([]config.DownloadPreset, 0, len(builtinDownloadPresets)+len(m.cfg.DownloadPresets))
	presets = append(presets, builtinDownloadPresets...)
	presets = append(presets, m.cfg.DownloadPresets...)
	return presets
}

// applyDownloadPreset fills the picker fields from a preset. Manual changes
// after applying a preset (quality, subtitles, thumbnail, rate) still win —
// presets are a shortcut, not a lock.
func (m *Model) applyDownloadPreset(index int) {
	presets := m.downloadPresets()
	if index < 0 || index >= len(presets) {
		return
	}
	preset := presets[index]
	m.downloadPickerQuality = defaultQualityIndex(preset.Quality)
	m.downloadPickerSubs = preset.WriteSubs
	m.downloadPickerThumb = preset.EmbedThumbnail
	m.downloadPickerRate = 0
	m.downloadPickerRateValue = ""
	if preset.RateLimit != "" {
		for i, choice := range downloadRateLimitChoices {
			if choice == preset.RateLimit {
				m.downloadPickerRate = i
				return
			}
		}
		m.downloadPickerRateValue = preset.RateLimit
	}
}
