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
		downloadItem{d: downloader.Download{Title: "Music", Channel: "Lecture Notes", Status: downloader.StatusQueued}},
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
