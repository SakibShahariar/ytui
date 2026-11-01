package logging

import (
	"os"
	"path/filepath"
	"sync"
	"strings"
	"testing"
)

func TestSetupDisabledDoesNotCreateLog(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	closeLog, path, err := Setup("off")
	if err != nil { t.Fatal(err) }
	if path != "" { t.Fatalf("disabled log path = %q, want empty", path) }
	if err := closeLog(); err != nil { t.Fatal(err) }
	entries, err := os.ReadDir(cache)
	if err != nil { t.Fatal(err) }
	if len(entries) != 0 { t.Fatalf("disabled logging created files: %v", entries) }
}

func TestSetupCreatesPrivateLogAndWritesEntries(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	closeLog, path, err := Setup("debug")
	if err != nil { t.Fatal(err) }
	if err := closeLog(); err != nil { t.Fatal(err) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0o600 { t.Fatalf("log permissions = %04o, want 0600", got) }
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(data), "file logging enabled") { t.Fatalf("log missing startup entry: %s", data) }
}

func TestRotatingWriterRetainsRecentBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ytui.log")
	writer, err := newRotatingWriter(path, 10, 2)
	if err != nil { t.Fatal(err) }
	defer writer.Close()

	for _, value := range []string{"0123456789", "abcd", "1234567890", "z"} {
		if _, err := writer.Write([]byte(value)); err != nil { t.Fatalf("Write(%q): %v", value, err) }
	}
	if err := writer.Close(); err != nil { t.Fatal(err) }

	for name, want := range map[string]string{
		"ytui.log": "z",
		"ytui.log.1": "1234567890",
		"ytui.log.2": "abcd",
	} {
		got, err := os.ReadFile(filepath.Join(filepath.Dir(path), name))
		if err != nil { t.Fatalf("read %s: %v", name, err) }
		if string(got) != want { t.Errorf("%s = %q, want %q", name, got, want) }
	}
	if _, err := os.Stat(path+".3"); !os.IsNotExist(err) {
		t.Fatalf("unexpected third backup: stat error = %v", err)
	}
}

func TestRotatingWriterSerializesConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ytui.log")
	writer, err := newRotatingWriter(path, 1024, 3)
	if err != nil { t.Fatal(err) }
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				if _, err := writer.Write([]byte(strings.Repeat("x", 20)+"\n")); err != nil {
					t.Errorf("concurrent Write(): %v", err)
					return
				}
			}
		}()
	}
	workers.Wait()
	if err := writer.Close(); err != nil { t.Fatal(err) }
	if _, err := os.Stat(path); err != nil { t.Fatalf("active log missing: %v", err) }
}

func TestLogRotationSettingsValidateEnvironment(t *testing.T) {
	t.Setenv("YTUI_LOG_MAX_BYTES", "2048")
	t.Setenv("YTUI_LOG_BACKUPS", "2")
	maxBytes, backups, err := logRotationSettings()
	if err != nil { t.Fatal(err) }
	if maxBytes != 2048 || backups != 2 { t.Fatalf("settings = (%d, %d), want (2048, 2)", maxBytes, backups) }

	t.Setenv("YTUI_LOG_MAX_BYTES", "invalid")
	if _, _, err := logRotationSettings(); err == nil { t.Fatal("invalid max size unexpectedly accepted") }
}

func TestRedactCredentials(t *testing.T) {
	for _, input := range []string{
		"access_token=secret-value",
		"{\"client_secret\":\"secret-value\"}",
		"https://example.test/callback?code=abc&state=ok",
	} {
		got := Redact(input)
		if strings.Contains(got, "secret-value") || strings.Contains(got, "code=abc") {
			t.Errorf("Redact(%q) = %q, credential leaked", input, got)
		}
	}
}
