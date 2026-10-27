package ui

import "strings"

// dependencyWarning returns a non-blocking startup warning for tools required
// for search and playback. OAuth is intentionally not part of this check:
// local lists remain usable without signing in.
func dependencyWarning(lookPath func(string) (string, error)) string {
	missing := make([]string, 0, 2)
	for _, name := range []string{"mpv", "yt-dlp"} {
		if _, err := lookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return "missing dependencies: " + strings.Join(missing, ", ") + " (search/playback may not work)"
}
