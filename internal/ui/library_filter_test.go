package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/SakibShahariar/ytui/internal/downloader"
	"github.com/SakibShahariar/ytui/internal/store"
)

func TestHistoryFilterMatchesTitleAndChannel(t *testing.T) {
	items := []list.Item{
		historyItem{e: store.HistoryEntry{Title: "Cooking", Channel: "Alice Kitchen"}},
		historyItem{e: store.HistoryEntry{Title: "Travel vlog", Channel: "Bob"}},
	}
	m := Model{activeTab: tabHistory, libraryFilter: "ALICE"}
	got := m.filterLibraryItems(items)
	if len(got) != 1 || got[0].(historyItem).e.Title != "Cooking" {
		t.Fatalf("filtered history = %#v, want only Alice's video", got)
	}
}

func TestDownloadTextAndStatusFiltersCombine(t *testing.T) {
	items := []list.Item{
		downloadItem{d: downloader.Download{Title: "Alice video", Channel: "Alice", Status: downloader.StatusDownloading}},
		downloadItem{d: downloader.Download{Title: "Alice finished", Channel: "Alice", Status: downloader.StatusDone}},
		downloadItem{d: downloader.Download{Title: "Bob failed", Channel: "Bob", Status: downloader.StatusError}},
	}
	m := Model{activeTab: tabDownloads, libraryFilter: "alice", downloadStatusFilter: 1}
	got := m.filterLibraryItems(items)
	if len(got) != 1 || got[0].(downloadItem).d.Status != downloader.StatusDownloading {
		t.Fatalf("combined filter returned %#v, want only active Alice download", got)
	}
	m.downloadStatusFilter = 3
	if got := m.filterLibraryItems(items); len(got) != 0 {
		t.Fatalf("failed filter with Alice query returned %#v, want no matches", got)
	}
}

func TestDownloadStatusFilterLabels(t *testing.T) {
	for filter, want := range map[int]string{0: "all", 1: "active", 2: "finished", 3: "failed/resumable"} {
		if got := (Model{downloadStatusFilter: filter}).downloadStatusFilterLabel(); got != want {
			t.Errorf("filter %d label = %q, want %q", filter, got, want)
		}
	}
}
