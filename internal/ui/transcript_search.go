package ui

import (
	"strconv"
	"strings"
)

// searchTranscript returns the indices of transcript lines containing query,
// case-insensitively. An empty query yields no matches.
func searchTranscript(text, query string) []int {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	matches := make([]int, 0)
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), needle) {
			matches = append(matches, i)
		}
	}
	return matches
}

// transcriptLineSeconds parses the "[MM:SS]" or "[HH:MM:SS]" prefix written
// by the VTT parser so a matched line can seek playback to that point.
func transcriptLineSeconds(line string) (float64, bool) {
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start < 0 || end <= start+1 {
		return 0, false
	}
	parts := strings.Split(line[start+1:end], ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var seconds float64
	for _, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil || value < 0 {
			return 0, false
		}
		seconds = seconds*60 + value
	}
	return seconds, true
}
