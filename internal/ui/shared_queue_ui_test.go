package ui

import (
	"path/filepath"
	"testing"

	"github.com/SakibShahariar/ytui/internal/sharedqueue"
)

func TestLoadSharedQueueCommandAndAcknowledgement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "shared-queue.json")
	t.Setenv("YTUI_SHARED_QUEUE_PATH", path)
	if err := sharedqueue.Append(sharedqueue.Item{ID: "event-1", URL: "https://example.com/video", Title: "Example", Mode: "video", Quality: "720p"}); err != nil { t.Fatal(err) }
	msg, ok := loadSharedQueueCmd(true)().(sharedQueueMsg)
	if !ok { t.Fatalf("message type = %T, want sharedQueueMsg", loadSharedQueueCmd(true)()) }
	if msg.err != nil || !msg.manual || len(msg.items) != 1 || msg.items[0].ID != "event-1" {
		t.Fatalf("shared queue message = %#v", msg)
	}
	ack, ok := ackSharedQueueCmd([]string{"event-1"}, 1)().(sharedQueueAckMsg)
	if !ok || ack.err != nil || ack.count != 1 { t.Fatalf("ack message = %#v", ack) }
	items, err := sharedqueue.Read()
	if err != nil { t.Fatal(err) }
	if len(items) != 0 { t.Fatalf("items after ack = %#v, want empty", items) }
}

func TestSharedQueueOptionsMapVideoQualityAndAudio(t *testing.T) {
	video := sharedQueueOptions(sharedqueue.Item{Mode: "video", Quality: "720p"})
	if video.Format != "bestvideo[height<=720]+bestaudio/best[height<=720]" || video.ExtractAudio {
		t.Fatalf("720p options = %#v", video)
	}
	audio := sharedQueueOptions(sharedqueue.Item{Mode: "audio", AudioFormat: "opus"})
	if !audio.ExtractAudio || audio.AudioFormat != "opus" || audio.QualityLabel != "Audio only (opus)" {
		t.Fatalf("audio options = %#v", audio)
	}
	invalidAudio := sharedQueueOptions(sharedqueue.Item{Mode: "audio", AudioFormat: "--exec=malicious"})
	if invalidAudio.AudioFormat != "mp3" { t.Fatalf("invalid audio format was not replaced safely: %#v", invalidAudio) }
}
