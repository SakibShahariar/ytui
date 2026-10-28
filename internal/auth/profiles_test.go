package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestProfilesKeepLegacyDefaultAndIsolateTokens(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := ActiveProfile(); err != nil || got != "default" {
		t.Fatalf("initial ActiveProfile() = %q, %v; want default", got, err)
	}
	defaultToken := &oauth2.Token{AccessToken: "default-secret"}
	if err := saveToken(defaultToken); err != nil { t.Fatal(err) }
	if err := CreateProfile("work"); err != nil { t.Fatal(err) }
	if err := SelectProfile("work"); err != nil { t.Fatal(err) }
	if err := saveToken(&oauth2.Token{AccessToken: "work-secret"}); err != nil { t.Fatal(err) }
	got, err := loadToken()
	if err != nil { t.Fatal(err) }
	if got.AccessToken != "work-secret" { t.Fatalf("active token = %q, want work-secret", got.AccessToken) }
	if err := SelectProfile("default"); err != nil { t.Fatal(err) }
	got, err = loadToken()
	if err != nil { t.Fatal(err) }
	if got.AccessToken != "default-secret" { t.Fatalf("legacy default token = %q, want default-secret", got.AccessToken) }
	defaultPath, _ := tokenPathForProfile("default")
	workPath, _ := tokenPathForProfile("work")
	if defaultPath == workPath { t.Fatal("default and named profiles share a token path") }
	var raw map[string]any
	data, err := os.ReadFile(workPath)
	if err != nil { t.Fatal(err) }
	if err := json.Unmarshal(data, &raw); err != nil { t.Fatal(err) }
	if raw["access_token"] != "work-secret" { t.Fatalf("work token file contains unexpected token: %#v", raw) }
}

func TestCreateProfileValidatesNamesAndRestrictsPermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"", "default", "../escape", "two words", "-bad", strings.Repeat("a", 33)} {
		if err := CreateProfile(name); err == nil { t.Errorf("CreateProfile(%q) unexpectedly succeeded", name) }
	}
	if err := CreateProfile("personal_2"); err != nil { t.Fatal(err) }
	root, _ := profilesDir()
	for path, want := range map[string]os.FileMode{
		root: 0o700,
		filepath.Join(root, "personal_2"): 0o700,
	} {
		info, err := os.Stat(path)
		if err != nil { t.Fatal(err) }
		if got := info.Mode().Perm(); got != want { t.Errorf("%s permissions = %04o, want %04o", path, got, want) }
	}
}

func TestRemoveActiveProfileReturnsToDefaultAndProtectsDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := CreateProfile("temporary"); err != nil { t.Fatal(err) }
	if err := SelectProfile("temporary"); err != nil { t.Fatal(err) }
	if err := RemoveProfile("default"); err == nil { t.Fatal("RemoveProfile(default) unexpectedly succeeded") }
	if err := RemoveProfile("temporary"); err != nil { t.Fatal(err) }
	active, err := ActiveProfile()
	if err != nil { t.Fatal(err) }
	if active != "default" { t.Fatalf("active profile after removal = %q, want default", active) }
	if _, err := os.Stat(filepath.Join(mustProfilesDir(t), "temporary")); !os.IsNotExist(err) {
		t.Fatalf("removed profile directory still exists or stat failed unexpectedly: %v", err)
	}
}

func mustProfilesDir(t *testing.T) string {
	t.Helper()
	dir, err := profilesDir()
	if err != nil { t.Fatal(err) }
	return dir
}
