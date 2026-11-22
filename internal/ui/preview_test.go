package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rivo/uniseg"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestPreviewFooterRowsStayWithinPanel(t *testing.T) {
	title := "What A Yorker! Naseem Shah Bowls Pollard 🌟#DPWorldILT20 #DVVMLIE #AllInForCricket #Shorts M9X1K"
	v := ytdlp.Video{Title: title, Channel: "Sports TV", Duration: 37, ViewCount: 12000}
	const innerW = 40
	lines := previewFooterLines(v, innerW)
	if len(lines) != previewFooterRows-1 {
		t.Fatalf("footer rows = %d, want %d", len(lines), previewFooterRows-1)
	}
	for i, line := range lines {
		plain := strings.TrimRight(ansiRE.ReplaceAllString(line, ""), " ")
		width := uniseg.StringWidth(plain)
		if width > innerW {
			t.Errorf("row %d width = %d > %d: %q", i, width, innerW, plain)
		}
	}
	if !strings.Contains(ansiRE.ReplaceAllString(lines[2], ""), "─") {
		t.Fatal("separator missing from preview footer")
	}
}
