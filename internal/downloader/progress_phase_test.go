package downloader

import "testing"

func TestProgressPhaseForMetadataLines(t *testing.T) {
	tests := map[string]string{
		"[youtube] Extracting URL: https://example.com/v":    "Resolving URL",
		"[youtube] abc: Downloading webpage":                 "Fetching video info",
		"[youtube] abc: Downloading android player API JSON": "Fetching video info",
		"[youtube] abc: Downloading m3u8 information":        "Resolving formats",
		"[info] abc: Downloading 1 format(s): 395+251":       "Preparing formats",
		"[download] Destination: /tmp/a.mp4":                 "Preparing download",
		"[download]  45.0% of 10MiB":                         "Downloading",
	}
	for line, want := range tests {
		if got := progressPhaseForLine(line); got != want {
			t.Errorf("progressPhaseForLine(%q) = %q, want %q", line, got, want)
		}
	}
}
