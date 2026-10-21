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
