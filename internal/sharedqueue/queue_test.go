package sharedqueue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendReadAndAckSharedQueueItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "shared-queue.json")
	t.Setenv("YTUI_SHARED_QUEUE_PATH", path)
	first := Item{ID: "event-1", URL: "https://www.youtube.com/watch?v=abc", Title: "First", Mode: "video"}
	if err := Append(first); err != nil { t.Fatal(err) }
	if err := Append(first); err != nil { t.Fatalf("duplicate append: %v", err) }
	items, err := Read()
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ID != "event-1" { t.Fatalf("items = %#v", items) }
	if err := Append(Item{ID: "event-2", URL: "https://example.com/audio", Mode: "audio", AudioFormat: "mp3"}); err != nil { t.Fatal(err) }
	if err := Ack([]string{"event-1"}); err != nil { t.Fatal(err) }
	items, err = Read()
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ID != "event-2" { t.Fatalf("after ack items = %#v; expected event-2 preserved", items) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0o600 { t.Fatalf("queue file permissions = %04o, want 0600", got) }
	info, err = os.Stat(filepath.Dir(path))
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0o700 { t.Fatalf("queue directory permissions = %04o, want 0700", got) }
}

func TestReadMissingSharedQueueIsEmpty(t *testing.T) {
	t.Setenv("YTUI_SHARED_QUEUE_PATH", filepath.Join(t.TempDir(), "missing", "queue.json"))
	items, err := Read()
	if err != nil || len(items) != 0 { t.Fatalf("Read() = %#v, %v; want empty", items, err) }
}

func TestAppendRejectsUnsafeSharedQueueItems(t *testing.T) {
	t.Setenv("YTUI_SHARED_QUEUE_PATH", filepath.Join(t.TempDir(), "queue.json"))
	for _, item := range []Item{
		{ID: "bad", URL: "file:///etc/passwd"},
		{ID: "bad", URL: "https://user:pass@example.com/video"},
		{ID: "../bad", URL: "https://example.com/video"},
		{ID: "has space", URL: "https://example.com/video"},
		{ID: "event:1", URL: "https://example.com/video"},
		{ID: "bad", URL: "https://example.com/video", Mode: "shell"},
		{ID: "bad", URL: "http://127.0.0.1/admin"},
		{ID: "bad", URL: "http://localhost:8080/"},
		{ID: "bad", URL: "https://example.com/video", Title: "unsafe\nname"},
	} {
		if err := Append(item); err == nil { t.Errorf("Append(%#v) unexpectedly succeeded", item) }
	}
}

func TestReadAcceptsYtrawlVersionOneFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-queue.json")
	t.Setenv("YTUI_SHARED_QUEUE_PATH", path)
	fixture := []byte(`{
		"version": 1,
		"items": [{
			"id": "event-from-ytrawl",
			"url": "https://www.youtube.com/watch?v=abc",
			"title": "Panel item",
			"mode": "audio",
			"quality": "best",
			"audio_format": "opus",
			"created_at": "2026-10-28T01:23:45.000Z"
		}]
	}`)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, fixture, 0o600); err != nil { t.Fatal(err) }
	items, err := Read()
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ID != "event-from-ytrawl" || items[0].Mode != "audio" || items[0].AudioFormat != "opus" {
		t.Fatalf("parsed Ytrawl fixture = %#v", items)
	}
}

func TestCustomQueuePathDoesNotChangeExistingParentPermissions(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0o755); err != nil { t.Fatal(err) }
	if err := os.Chmod(parent, 0o755); err != nil { t.Fatal(err) }
	t.Setenv("YTUI_SHARED_QUEUE_PATH", filepath.Join(parent, "inbox.json"))
	if err := Append(Item{ID: "permission-check", URL: "https://example.com/video"}); err != nil { t.Fatal(err) }
	info, err := os.Stat(parent)
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0o755 { t.Fatalf("custom parent permissions = %04o, want unchanged 0755", got) }
	info, err = os.Stat(filepath.Join(parent, "inbox.json"))
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0o600 { t.Fatalf("queue file permissions = %04o, want 0600", got) }
}

func TestReadRejectsDuplicateIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-queue.json")
	t.Setenv("YTUI_SHARED_QUEUE_PATH", path)
	if err := os.WriteFile(path, []byte(`{"version":1,"items":[{"id":"same","url":"https://example.com/one"},{"id":"same","url":"https://example.com/two"}]}`), 0o600); err != nil { t.Fatal(err) }
	if _, err := Read(); err == nil { t.Fatal("Read accepted duplicate item IDs") }
}
