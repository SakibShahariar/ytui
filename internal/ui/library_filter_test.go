package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/library"
	"github.com/SakibShahariar/ytui/internal/store"
)

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
