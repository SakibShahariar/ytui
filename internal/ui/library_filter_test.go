package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/library"
	"github.com/SakibShahariar/ytui/internal/store"
)

func TestSameDownloadRowsSkipsUnchangedVisibleRows(t *testing.T) {
	current := []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20, Speed: "1MiB/s", ETA: "10s"}}}
	next := []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20.04, Speed: "1MiB/s", ETA: "10s"}}}
	if !sameDownloadRows(current, next) { t.Fatal("same rendered rows were treated as changed") }
}

func TestSameDownloadRowsDetectsVisibleChanges(t *testing.T) {
	base := []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20, Speed: "1MiB/s", ETA: "10s", Opts: downloader.Options{QualityLabel: "1080p"}}}}
	tests := []struct { name string; next []list.Item }{
		{name: "progress", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 30, Speed: "1MiB/s", ETA: "10s", Opts: downloader.Options{QualityLabel: "1080p"}}}}},
		{name: "speed", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20, Speed: "2MiB/s", ETA: "10s", Opts: downloader.Options{QualityLabel: "1080p"}}}}},
		{name: "eta", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20, Speed: "1MiB/s", ETA: "5s", Opts: downloader.Options{QualityLabel: "1080p"}}}}},
		{name: "status", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDone, Progress: 100, FilePath: "/tmp/Lecture.mp4"}}}},
		{name: "title", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture 2", Status: downloader.StatusDownloading, Progress: 20, Speed: "1MiB/s", ETA: "10s", Opts: downloader.Options{QualityLabel: "1080p"}}}}},
		{name: "quality", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusDownloading, Progress: 20, Speed: "1MiB/s", ETA: "10s", Opts: downloader.Options{QualityLabel: "720p"}}}}},
		{name: "error", next: []list.Item{downloadItem{d: downloader.Download{Title: "Lecture", Status: downloader.StatusError, Err: errors.New("disk full")}}}},
		{name: "row count", next: nil},
		{name: "row type", next: []list.Item{historyItem{}}},
	}
	for _, tt := range tests { t.Run(tt.name, func(t *testing.T) { if sameDownloadRows(base, tt.next) { t.Fatal("visibly changed rows were treated as unchanged") } }) }
}

func TestFilterLibraryItemsMatchesHistoryTitleAndChannelCaseInsensitively(t *testing.T) {
	m := Model{activeTab: tabHistory, libraryFilter: "jazz"}
	items := []list.Item{
		historyItem{e: store.HistoryEntry{Title: "Late Night Mix", Channel: "Jazz Room", WatchedAt: time.Now()}},
		historyItem{e: store.HistoryEntry{Title: "Jazz Piano", Channel: "Quiet Keys", WatchedAt: time.Now()}},
		historyItem{e: store.HistoryEntry{Title: "Rock Classics", Channel: "Guitar Room", WatchedAt: time.Now()}},
	}
	got := m.filterLibraryItems(items)
	if len(got) != 2 {
		t.Fatalf("filterLibraryItems() returned %d items, want 2", len(got))
	}
	if got[0].(historyItem).e.Title != "Late Night Mix" || got[1].(historyItem).e.Title != "Jazz Piano" {
		t.Fatalf("filtered history = %#v, want the two items matching title or channel", got)
	}
}

func TestFilterLibraryItemsFiltersDownloadsByStatusAndText(t *testing.T) {
	m := Model{activeTab: tabDownloads, libraryFilter: "lecture", downloadStatusFilter: 1}
	items := []list.Item{
		downloadItem{d: downloader.Download{Title: "Lecture 1", Channel: "Course", Status: downloader.StatusDownloading}},
		downloadItem{d: downloader.Download{Title: "Lecture 2", Channel: "Course", Status: downloader.StatusDone}},
		downloadItem{d: downloader.Download{Title: "Music", Channel: "Jazz Course", Status: downloader.StatusQueued}},
	}
	got := m.filterLibraryItems(items)
	if len(got) != 1 {
		t.Fatalf("active lecture filter returned %d items, want 1", len(got))
	}
	if got[0].(downloadItem).d.Title != "Lecture 1" {
		t.Fatalf("filtered download = %q, want Lecture 1", got[0].(downloadItem).d.Title)
	}

	m.libraryFilter = ""
	m.downloadStatusFilter = 2
	got = m.filterLibraryItems(items)
	if len(got) != 1 || got[0].(downloadItem).d.Title != "Lecture 2" {
		t.Fatalf("finished filter = %#v, want only Lecture 2", got)
	}

	m.downloadStatusFilter = 3
	items = append(items, downloadItem{d: downloader.Download{Title: "Lecture 3", Status: downloader.StatusResumable}})
	got = m.filterLibraryItems(items)
	if len(got) != 1 || got[0].(downloadItem).d.Title != "Lecture 3" {
		t.Fatalf("failed/resumable filter = %#v, want only Lecture 3", got)
	}
}

func TestFilterLibraryItemsSearchesOfflineLibraryNames(t *testing.T) {
	m := Model{activeTab: tabDownloads, offlineDownloads: true, libraryFilter: "concert"}
	items := []list.Item{
		localDownloadItem{file: library.File{Name: "Live Concert.webm"}},
		localDownloadItem{file: library.File{Name: "Lecture.mp4"}},
	}
	got := m.filterLibraryItems(items)
	if len(got) != 1 || got[0].(localDownloadItem).file.Name != "Live Concert.webm" {
		t.Fatalf("offline filter = %#v, want only Live Concert.webm", got)
	}
}


func TestFilterLibraryItemsByDateRange(t *testing.T) {
	now := time.Now()
	m := Model{activeTab: tabHistory, libraryDateFilter: 1}
	history := []list.Item{
		historyItem{e: store.HistoryEntry{Title: "Recent", WatchedAt: now.Add(-24 * time.Hour)}},
		historyItem{e: store.HistoryEntry{Title: "Old", WatchedAt: now.Add(-10 * 24 * time.Hour)}},
	}
	got := m.filterLibraryItems(history)
	if len(got) != 1 || got[0].(historyItem).e.Title != "Recent" {
		t.Fatalf("7-day history filter = %#v, want only Recent", got)
	}

	m = Model{activeTab: tabDownloads, libraryDateFilter: 1}
	downloads := []list.Item{
		downloadItem{d: downloader.Download{Title: "Recent", Status: downloader.StatusQueued, AddedAt: now.Add(-time.Hour)}},
		downloadItem{d: downloader.Download{Title: "Old", Status: downloader.StatusPaused, AddedAt: now.Add(-8 * 24 * time.Hour)}},
	}
	got = m.filterLibraryItems(downloads)
	if len(got) != 1 || got[0].(downloadItem).d.Title != "Recent" {
		t.Fatalf("7-day download filter = %#v, want only Recent", got)
	}

	m = Model{activeTab: tabDownloads, offlineDownloads: true, libraryDateFilter: 1}
	offline := []list.Item{
		localDownloadItem{file: library.File{Name: "Recent.mp4", ModTime: now.Add(-time.Hour)}},
		localDownloadItem{file: library.File{Name: "Old.mp4", ModTime: now.Add(-8 * 24 * time.Hour)}},
	}
	got = m.filterLibraryItems(offline)
	if len(got) != 1 || got[0].(localDownloadItem).file.Name != "Recent.mp4" {
		t.Fatalf("7-day offline filter = %#v, want only Recent.mp4", got)
	}
}

func TestActiveDownloadFilterIncludesPausedQueueEntries(t *testing.T) {
	m := Model{activeTab: tabDownloads, downloadStatusFilter: 1}
	items := []list.Item{
		downloadItem{d: downloader.Download{Title: "Queued", Status: downloader.StatusQueued}},
		downloadItem{d: downloader.Download{Title: "Paused", Status: downloader.StatusPaused}},
		downloadItem{d: downloader.Download{Title: "Done", Status: downloader.StatusDone}},
	}
	got := m.filterLibraryItems(items)
	if len(got) != 2 { t.Fatalf("active filter returned %d items, want queued and paused", len(got)) }
}


func TestSortDownloadItemsSupportsQueueSortModes(t *testing.T) {
	base := []list.Item{
		downloadItem{d: downloader.Download{Title: "Beta", Channel: "Zulu", Status: downloader.StatusDone, Progress: 20, AddedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}},
		downloadItem{d: downloader.Download{Title: "Alpha", Channel: "Alpha", Status: downloader.StatusQueued, Progress: 50, AddedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		downloadItem{d: downloader.Download{Title: "Gamma", Channel: "Mike", Status: downloader.StatusPaused, Progress: 10, AddedAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)}},
	}
	tests := []struct {
		mode string
		want []string
	}{
		{mode: "added", want: []string{"Alpha", "Beta", "Gamma"}},
		{mode: "title", want: []string{"Alpha", "Beta", "Gamma"}},
		{mode: "channel", want: []string{"Alpha", "Gamma", "Beta"}},
		{mode: "progress", want: []string{"Alpha", "Beta", "Gamma"}},
		{mode: "status", want: []string{"Alpha", "Gamma", "Beta"}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			items := append([]list.Item(nil), base...)
			sortDownloadItems(items, tt.mode)
			for i, want := range tt.want {
				if got := items[i].(downloadItem).d.Title; got != want {
					t.Fatalf("sorted item %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}


func TestDownloadOptionsSummaryIncludesSelectedOptions(t *testing.T) {
	got := downloadOptionsSummary(downloader.Options{
		QualityLabel: "1080p",
		WriteSubs: true,
		EmbedThumbnail: true,
		CookiesFromBrowser: "firefox",
	})
	want := "1080p · subtitles · thumbnail · cookies: firefox"
	if got != want {
		t.Fatalf("downloadOptionsSummary() = %q, want %q", got, want)
	}
}

func TestCompletedDownloadDescriptionShowsOptions(t *testing.T) {
	item := downloadItem{d: downloader.Download{
		Title: "Example",
		Status: downloader.StatusDone,
		FilePath: "/downloads/example.mp4",
		Opts: downloader.Options{QualityLabel: "Audio only (mp3)", WriteSubs: true},
	}}
	got := item.Description()
	if !strings.Contains(got, "Audio only (mp3)") || !strings.Contains(got, "subtitles") || !strings.Contains(got, "/downloads/example.mp4") {
		t.Fatalf("completed download description = %q, want options and output path", got)
	}
}


func TestDownloadingDescriptionShowsPhaseAndFileSize(t *testing.T) {
	item := downloadItem{d: downloader.Download{
		Status: downloader.StatusDownloading,
		Progress: 42.5,
		Speed: "5MiB/s",
		ETA: "00:12",
		FileSize: "123MiB",
		Phase: "Merging formats",
		Opts: downloader.Options{QualityLabel: "1080p"},
	}}
	got := item.Description()
	for _, want := range []string{"1080p", "Merging formats", "123MiB", "42.5%", "5MiB/s", "00:12"} {
		if !strings.Contains(got, want) {
			t.Errorf("download description %q does not include %q", got, want)
		}
	}
}
