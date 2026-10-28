package logging

import (
	"os"
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
