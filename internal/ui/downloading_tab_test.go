package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/downloader"
)

func downloadingTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	manager, err := downloader.NewWithConcurrency(filepath.Join(t.TempDir(), "downloads"), 1)
	if err != nil {
		t.Fatal(err)
	}
	return Model{cfg: config.Config{}, downloads: manager}
}

func TestDownloadingTabOnlyVisibleWhenActive(t *testing.T) {
	m := downloadingTestModel(t)
	for _, tab := range m.visibleTabs() {
		if tab == tabDownloading {
			t.Fatal("Downloading tab visible while idle")
		}
	}
	queued := m.downloads.Enqueue("v1", "Video", "Channel", "https://example.com/v", downloader.Options{})
	_ = queued
	for _, tab := range m.visibleTabs() {
		if tab == tabDownloading {
			return
		}
	}
	t.Fatal("Downloading tab missing while a transfer is queued")
}

func TestDownloadingTabShowsOnlyActiveItems(t *testing.T) {
	m := downloadingTestModel(t)
	// Keep the fake transfer alive so it cannot finish before the assertion.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	manager, err := downloader.NewWithConcurrency(filepath.Join(t.TempDir(), "downloads"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()
	m.downloads = manager
	m.downloads.Enqueue("v1", "Active", "Channel", "https://example.com/v", downloader.Options{})
	m.activeTab = tabDownloading
	m.list = list.New(nil, ytDelegate{}, 60, 9)
	items := m.currentListItems()
	if len(items) != 1 || items[0].(downloadItem).d.VideoID != "v1" {
		t.Fatalf("active items = %v", items)
	}
}

func TestDownloadingTabClosesWhenTransfersFinish(t *testing.T) {
	m := downloadingTestModel(t)
	m.activeTab = tabDownloading
	m.list = list.New(nil, ytDelegate{}, 60, 9)
	updated, _ := m.Update(downloadsTickMsg{})
	result := updated.(Model)
	if result.activeTab != tabDownloads {
		t.Fatalf("active tab after completion = %v, want Downloads", result.activeTab)
	}
}

func TestDownloadingTabVisibleWhileViewed(t *testing.T) {
	m := downloadingTestModel(t)
	m.activeTab = tabDownloading
	found := false
	for _, tab := range m.visibleTabs() {
		if tab == tabDownloading {
			found = true
		}
	}
	if !found {
		t.Fatal("Downloading tab disappeared while still being viewed")
	}
}
