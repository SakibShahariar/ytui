package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedBackupFiles(t *testing.T, base string) (configPath, dataPath, queuePath string) {
	t.Helper()
	configPath = filepath.Join(base, "config", "config.json")
	dataPath = filepath.Join(base, "data", "data.json")
	downloadDir := filepath.Join(base, "downloads")
	queuePath = filepath.Join(downloadDir, ".ytui-queue.json")
	for path, content := range map[string]string{
		configPath: `{"download_rate_limit":"500K"}`,
		dataPath:   `{"subscriptions":[],"history":[],"watch_later":[{"video_id":"abc","title":"Saved"}]}`,
		queuePath:  `[{"video_id":"queued","title":"Queued","status":0}]`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("YTUI_CONFIG_PATH", configPath)
	t.Setenv("YTUI_DATA_PATH", dataPath)
	t.Setenv("YTUI_DOWNLOAD_DIR", downloadDir)
	return
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	base := t.TempDir()
	configPath, dataPath, queuePath := seedBackupFiles(t, base)
	bundlePath := filepath.Join(base, "backup.json")
	var out, errOut strings.Builder
	if err := runBackupCommand([]string{bundlePath}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"token", "secret", "client_id", "cookie"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("backup contains credential key %q", forbidden)
		}
	}

	// Point every path at an empty location, then restore.
	dest := t.TempDir()
	t.Setenv("YTUI_CONFIG_PATH", filepath.Join(dest, "config", "config.json"))
	t.Setenv("YTUI_DATA_PATH", filepath.Join(dest, "data", "data.json"))
	t.Setenv("YTUI_DOWNLOAD_DIR", filepath.Join(dest, "downloads"))
	out.Reset()
	if err := runRestoreCommand([]string{bundlePath, "--yes"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dest, "config", "config.json"),
		filepath.Join(dest, "data", "data.json"),
		filepath.Join(dest, "downloads", ".ytui-queue.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("restored file missing: %v", err)
		}
	}
	restored, err := os.ReadFile(filepath.Join(dest, "data", "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restored), "Saved") {
		t.Fatalf("restored library = %q", restored)
	}
	_ = configPath // seeded via env; restored values verified above
	_ = dataPath
	_ = queuePath
}

func TestBackupCanSelectContents(t *testing.T) {
	base := t.TempDir()
	seedBackupFiles(t, base)
	bundlePath := filepath.Join(base, "selection.json")
	var out, errOut strings.Builder
	if err := runBackupCommand([]string{bundlePath, "--no-queue", "--yes"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	var bundle backupBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Config) == 0 || len(bundle.Store) == 0 || len(bundle.DownloadQueue) != 0 {
		t.Fatalf("selected backup = config %v, store %v, queue %v", bundle.Config != nil, bundle.Store != nil, bundle.DownloadQueue != nil)
	}
}

func TestBackupRejectsInvalidData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":999}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	err := runRestoreCommand([]string{path, "--yes"}, &out, &errOut)
	var exitErr *exitError
	if !errors.As(err, &exitErr) || exitErr.code != 1 {
		t.Fatalf("error = %v, want exit code 1", err)
	}
}
