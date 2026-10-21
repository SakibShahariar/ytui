package ui

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestWrapTwoLinesTruncatesByDisplayWidth(t *testing.T) {
	const width = 8
	_, line2 := wrapTwoLines(strings.Repeat("界", 30), width)
	if got := uniseg.StringWidth(line2); got > width {
		t.Errorf("second line display width = %d, want at most %d (%q)", got, width, line2)
	}
}
