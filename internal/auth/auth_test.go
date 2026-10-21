package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportClientCredentialsUsesPrivateFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := ImportClientCredentials("client-id", "client-secret"); err != nil {
		t.Fatalf("ImportClientCredentials() error = %v", err)
	}
	path := ClientSecretsPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat client secrets: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("client secret permissions = %04o, want 0600", got)
	}
}

func TestImportClientCredentialsDoesNotFollowPredictableTempSymlink(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	dir := filepath.Join(configHome, "ytui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "do-not-overwrite.txt")
	const original = "preserve this file"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "client_secret.json.tmp")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := ImportClientCredentials("client-id", "client-secret"); err != nil {
		t.Fatalf("ImportClientCredentials() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("symlink target contents = %q, want %q", got, original)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("predictable temp symlink unexpectedly changed: %v", err)
	}
}

func TestWritePrivateAtomicProtectsOAuthToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	if err := writePrivateAtomic(path, []byte(`{"access_token":"secret"}`)); err != nil {
		t.Fatalf("writePrivateAtomic() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token permissions = %04o, want 0600", got)
	}
}
