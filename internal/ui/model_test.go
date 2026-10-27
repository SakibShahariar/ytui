package ui

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestWrapTwoLinesTruncatesByDisplayWidth(t *testing.T) {
	const width = 8
	_, line2 := wrapTwoLines(strings.Repeat("界 ", 30), width)
	if got := uniseg.StringWidth(line2); got > width {
		t.Errorf("second line display width = %d, want at most %d (%q)", got, width, line2)
	}
}


func TestBrowserConfigPathsForOS(t *testing.T) {
	tests := []struct {
		goos string
		name string
		want string
	}{
		{"linux", "firefox", ".mozilla/firefox"},
		{"darwin", "chrome", "Library/Application Support/Google/Chrome"},
		{"windows", "edge", "AppData/Local/Microsoft/Edge/User Data"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.name, func(t *testing.T) {
			if got := browserConfigPathsForOS(tt.goos)[tt.name]; got != tt.want {
				t.Fatalf("browserConfigPathsForOS(%q)[%q] = %q, want %q", tt.goos, tt.name, got, tt.want)
			}
		})
	}
}
