package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/auth"
	"github.com/SakibShahariar/ytui/internal/config"
)

func TestValidateVideoURL(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
		ok    bool
	}{
		{"https://www.youtube.com/watch?v=abc123", "https://www.youtube.com/watch?v=abc123", true},
		{" https://example.com/video.mp4 ", "https://example.com/video.mp4", true},
		{"", "", false},
		{"not a URL", "", false},
		{"file:///tmp/video.mp4", "", false},
		{"https:///missing-host", "", false},
		{"https://user:pass@example.com/video", "", false},
	} {
		got, err := validateVideoURL(tc.input)
		if (err == nil) != tc.ok {
			t.Errorf("validateVideoURL(%q) error = %v, want success=%v", tc.input, err, tc.ok)
		}
		if err == nil && got != tc.want {
			t.Errorf("validateVideoURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRunSubcommandHelpAndUnknownFallback(t *testing.T) {
	var output bytes.Buffer
	handled, err := runSubcommand([]string{"help"}, strings.NewReader(""), &output, &output, false)
	if err != nil || !handled {
		t.Fatalf("runSubcommand(help) = (%v, %v), want handled without error", handled, err)
	}
	for _, want := range []string{"doctor", "profile list", "config export PATH", "--config PATH", "--data PATH"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("help output missing %q", want)
		}
	}
	handled, err = runSubcommand([]string{"some-url-or-argument"}, strings.NewReader(""), &output, &output, false)
	if err != nil || handled {
		t.Fatalf("unknown argument should fall through to TUI: handled=%v err=%v", handled, err)
	}
}

func TestConfigSubcommandsExportAndImport(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "active.json")
	t.Setenv("YTUI_CONFIG_PATH", activePath)
	if err := config.Save(config.Config{DownloadDir: filepath.Join(t.TempDir(), "downloads")}); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(t.TempDir(), "portable.json")
	var output bytes.Buffer
	if err := runConfigSubcommand([]string{"export", exportPath}, strings.NewReader(""), &output, false); err != nil {
		t.Fatalf("config export: %v", err)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("export file missing: %v", err)
	}
	if err := config.Save(config.Config{DownloadDir: filepath.Join(t.TempDir(), "other")}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSubcommand([]string{"import", exportPath}, strings.NewReader("yes\n"), &output, false); err != nil {
		t.Fatalf("config import: %v", err)
	}
	got, err := config.LoadWithError()
	if err != nil {
		t.Fatal(err)
	}
	if got.DownloadDir == "" || strings.Contains(got.DownloadDir, "other") {
		t.Fatalf("imported config DownloadDir = %q, want exported settings", got.DownloadDir)
	}
	if !strings.Contains(output.String(), "Exported settings") || !strings.Contains(output.String(), "Imported settings") {
		t.Fatalf("subcommand output = %q", output.String())
	}
}

func TestConfigImportSubcommandHonorsYesFlag(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "active.json")
	t.Setenv("YTUI_CONFIG_PATH", activePath)
	source := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(source, []byte("{\"download_dir\":\"portable-downloads\"}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runConfigSubcommand([]string{"import", source, "--yes"}, strings.NewReader(""), &output, false); err != nil {
		t.Fatalf("config import --yes: %v", err)
	}
	got, err := config.LoadWithError()
	if err != nil {
		t.Fatal(err)
	}
	if got.DownloadDir != "portable-downloads" {
		t.Fatalf("DownloadDir = %q, want imported value", got.DownloadDir)
	}
}

func TestProfileSubcommandsCreateSelectListAndRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var output bytes.Buffer
	if err := runProfileSubcommand([]string{"create", "work"}, strings.NewReader(""), &output, false); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := runProfileSubcommand([]string{"use", "work"}, strings.NewReader(""), &output, false); err != nil {
		t.Fatalf("select profile: %v", err)
	}
	active, err := auth.ActiveProfile()
	if err != nil || active != "work" { t.Fatalf("active profile = %q, %v; want work", active, err) }
	output.Reset()
	if err := runProfileSubcommand([]string{"list"}, strings.NewReader(""), &output, false); err != nil { t.Fatal(err) }
	if !strings.Contains(output.String(), "* work (no token)") || !strings.Contains(output.String(), "default") {
		t.Fatalf("profile list output = %q", output.String())
	}
	output.Reset()
	if err := runProfileSubcommand([]string{"remove", "work"}, strings.NewReader("n\n"), &output, false); err != nil { t.Fatal(err) }
	active, err = auth.ActiveProfile()
	if err != nil || active != "work" { t.Fatalf("cancelled removal changed active profile to %q (%v)", active, err) }
	if err := runProfileSubcommand([]string{"remove", "work", "--yes"}, strings.NewReader(""), &output, false); err != nil { t.Fatal(err) }
	active, err = auth.ActiveProfile()
	if err != nil || active != "default" { t.Fatalf("active profile after removal = %q, %v; want default", active, err) }
}

func TestProfileSubcommandRemovalConfirmation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := auth.CreateProfile("personal"); err != nil { t.Fatal(err) }
	var output bytes.Buffer
	if err := runProfileSubcommand([]string{"remove", "personal"}, strings.NewReader("yes\n"), &output, false); err != nil { t.Fatal(err) }
	if !strings.Contains(output.String(), "Removed profile") { t.Fatalf("output = %q", output.String()) }
}
