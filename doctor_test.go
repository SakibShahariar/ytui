package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderDoctorReport(t *testing.T) {
	var output bytes.Buffer
	report := doctorReport{
		version: "v1.2.3",
		configPath: "/home/test/.config/ytui/config.json",
		configStatus: "valid",
		dataPath: "/home/test/.config/ytui/data.json",
		dataStatus: "file exists (12 bytes)",
		authStatus: "token present (contents not read or displayed)",
		downloadPath: "/home/test/Downloads/ytui",
		downloadState: "directory exists",
		tools: []doctorTool{
			{name: "mpv", path: "/usr/bin/mpv", version: "mpv 0.39.0", status: "available"},
			{name: "yt-dlp", status: "missing from PATH"},
		},
	}
	if err := renderDoctorReport(&output, report); err != nil {
		t.Fatalf("renderDoctorReport() error = %v", err)
	}
	got := output.String()
	for _, want := range []string{
		"ytui doctor",
		"Version: v1.2.3",
		"config.json (valid)",
		"Authentication: token present (contents not read or displayed)",
		"mpv: available; path: /usr/bin/mpv; version: mpv 0.39.0",
		"yt-dlp: missing from PATH",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor report missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "client_secret") || strings.Contains(got, "access_token=") {
		t.Fatalf("doctor report appears to expose credentials: %s", got)
	}
}

func TestCleanDiagnosticRemovesControlCharactersAndLimitsLength(t *testing.T) {
	got := cleanDiagnostic(" version\nsecret\x1b[31m" + strings.Repeat("x", 200))
	if strings.ContainsAny(got, "\n\r\x1b") {
		t.Fatalf("cleanDiagnostic() retained control characters: %q", got)
	}
	if len([]rune(got)) > 161 {
		t.Fatalf("cleanDiagnostic() length = %d, want at most 161 runes", len([]rune(got)))
	}
}
