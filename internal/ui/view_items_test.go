package ui

import (
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/ytdlp"
)

func TestVideoItemOmitsUnknownViewCount(t *testing.T) {
	unknown := videoItem{v: ytdlp.Video{Channel: "Gigguk"}}
	if got := unknown.Description(); strings.Contains(got, "views") {
		t.Fatalf("unknown view count rendered as %q", got)
	}
	known := videoItem{v: ytdlp.Video{Channel: "Gigguk", ViewCount: 98575}}
	if got := known.Description(); !strings.Contains(got, "98.6K views") {
		t.Fatalf("known view count rendered as %q", got)
	}
}
