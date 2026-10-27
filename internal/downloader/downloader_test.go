package downloader

import "testing"

func TestProgressRegex(t *testing.T) {
	line := "[download]  42.3% of  123.45MiB at  5.67MiB/s ETA 00:12"
	match := progressRe.FindStringSubmatch(line)
	if len(match) != 4 {
		t.Fatalf("progress regex did not match sample: %q", line)
	}
	if match[1] != "42.3" || match[2] != "5.67MiB/s" || match[3] != "00:12" {
		t.Errorf("unexpected progress captures: %#v", match[1:])
	}
}

func TestDestinationRegex(t *testing.T) {
	line := "[download] Destination: /tmp/video title.webm"
	match := destRe.FindStringSubmatch(line)
	if len(match) != 2 {
		t.Fatalf("destination regex did not match sample: %q", line)
	}
	if match[1] != "/tmp/video title.webm" {
		t.Errorf("destination = %q, want full path", match[1])
	}
}

func TestRunDoesNotRestartCanceledDownload(t *testing.T) {
	d := &Download{VideoID: "video", Status: StatusCanceled}
	m := &Manager{
		byID:    map[string]*Download{"video": d},
		order:   []string{"video"},
		destDir: t.TempDir(),
	}

	m.run(d)
	if d.Status != StatusCanceled {
		t.Errorf("status after run = %v, want canceled", d.Status)
	}
	if d.cmd != nil {
		t.Error("canceled download unexpectedly created a process")
	}
}


func TestShutdownPreventsNewDownloads(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Shutdown()
	d := m.Enqueue("video", "title", "channel", "https://example.com/video", Options{})
	if d.Status != StatusCanceled {
		t.Fatalf("Enqueue() after Shutdown status = %v, want canceled", d.Status)
	}
	if m.Active() {
		t.Fatal("manager reports active downloads after Shutdown")
	}
}
