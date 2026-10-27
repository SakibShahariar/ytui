package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestDependencyWarning(t *testing.T) {
	tests := []struct {
		name    string
		missing map[string]bool
		want    string
	}{
		{name: "all available", missing: map[string]bool{}, want: ""},
		{name: "mpv missing", missing: map[string]bool{"mpv": true}, want: "mpv"},
		{name: "yt-dlp missing", missing: map[string]bool{"yt-dlp": true}, want: "yt-dlp"},
		{name: "both missing", missing: map[string]bool{"mpv": true, "yt-dlp": true}, want: "mpv, yt-dlp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dependencyWarning(func(name string) (string, error) {
				if tt.missing[name] {
					return "", errors.New("not found")
				}
				return "/fake/bin/" + name, nil
			})
			if tt.want == "" {
				if got != "" {
					t.Fatalf("dependencyWarning() = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("dependencyWarning() = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}
