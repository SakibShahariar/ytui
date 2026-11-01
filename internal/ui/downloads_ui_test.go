package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/SakibShahariar/ytui/internal/downloader"
)

func TestCancelSelectedDownloadRequestsConfirmation(t *testing.T) {
	item := downloadItem{d: downloader.Download{VideoID: "cancel-video", Title: "A video", Status: downloader.StatusDownloading}}
	m := Model{list: list.New([]list.Item{item}, list.NewDefaultDelegate(), 40, 10)}
	updated, cmd := m.cancelSelectedDownload()
	got := updated.(Model)
	if cmd != nil { t.Fatalf("cancelSelectedDownload() command = %v, want nil", cmd) }
	if got.downloadCancelPending == nil { t.Fatal("cancellation prompt was not created") }
	if got.downloadCancelPending.videoID != "cancel-video" || got.downloadCancelPending.title != "A video" {
		t.Fatalf("pending cancellation = %#v", got.downloadCancelPending)
	}
	if got.statusMsg != "cancel A video? press y to confirm, any other key to dismiss" {
		t.Fatalf("prompt status = %q", got.statusMsg)
	}
}

func TestResolveDownloadCancelDismissesWithoutCanceling(t *testing.T) {
	called := false
	m := Model{downloadCancelPending: &downloadCancelRequest{videoID: "cancel-video", title: "A video"}}
	got := m.resolveDownloadCancel("n", func(string) bool { called = true; return true })
	if called { t.Fatal("dismissal invoked cancellation") }
	if got.downloadCancelPending != nil { t.Fatal("dismissal left cancellation pending") }
	if got.statusMsg != "download cancellation dismissed" { t.Fatalf("status = %q", got.statusMsg) }
}

func TestResolveDownloadCancelRequiresExplicitConfirmation(t *testing.T) {
	canceledID := ""
	m := Model{downloadCancelPending: &downloadCancelRequest{videoID: "cancel-video", title: "A video"}}
	got := m.resolveDownloadCancel("y", func(videoID string) bool { canceledID = videoID; return true })
	if canceledID != "cancel-video" { t.Fatalf("canceled video ID = %q, want cancel-video", canceledID) }
	if got.downloadCancelPending != nil { t.Fatal("confirmation left cancellation pending") }
	if got.statusMsg != "canceled: A video" { t.Fatalf("status = %q", got.statusMsg) }
}

func TestResolveDownloadCancelDoesNotReportCancellationAfterCompletion(t *testing.T) {
	m := Model{downloadCancelPending: &downloadCancelRequest{videoID: "cancel-video", title: "A video"}}
	got := m.resolveDownloadCancel("y", func(string) bool { return false })
	if got.downloadCancelPending != nil { t.Fatal("completed download left cancellation pending") }
	if got.statusMsg != "download is no longer active" { t.Fatalf("status = %q", got.statusMsg) }
}

func TestResolveDownloadCancelHandlesUnavailableManager(t *testing.T) {
	m := Model{downloadCancelPending: &downloadCancelRequest{videoID: "cancel-video", title: "A video"}}
	got := m.resolveDownloadCancel("y", nil)
	if got.downloadCancelPending != nil { t.Fatal("unavailable manager left cancellation pending") }
	if got.statusMsg != "downloads unavailable" { t.Fatalf("status = %q", got.statusMsg) }
}
