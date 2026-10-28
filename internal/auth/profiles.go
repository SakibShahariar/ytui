package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultProfileName = "default"

// ProfileInfo reports profile metadata without exposing any credential material.
type ProfileInfo struct {
	Name    string
	Active  bool
	HasToken bool
}

func validProfileName(name string) bool {
	if name == "" || len(name) > 32 || name == defaultProfileName { return false }
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (i > 0 && (r == '-' || r == '_')) {
			continue
		}
		return false
	}
	return true
}

func profilesDir() (string, error) {
	dir, err := configDir()
	if err != nil { return "", err }
	return filepath.Join(dir, "profiles"), nil
}

func profileDir(name string) (string, error) {
	if name == defaultProfileName { return configDir() }
	if !validProfileName(name) { return "", fmt.Errorf("invalid profile name %q (use 1–32 letters/digits, with optional - or _ after the first character)", name) }
	root, err := profilesDir()
	if err != nil { return "", err }
	return filepath.Join(root, name), nil
}

// ActiveProfile returns the currently selected account profile. If no
// selection file exists, the legacy single-profile setup remains active.
func ActiveProfile() (string, error) {
	dir, err := configDir()
	if err != nil { return "", err }
	data, err := os.ReadFile(filepath.Join(dir, "active_profile"))
	if os.IsNotExist(err) { return defaultProfileName, nil }
	if err != nil { return "", fmt.Errorf("read active profile: %w", err) }
	name := strings.TrimSpace(string(data))
	if name == defaultProfileName { return name, nil }
	if !validProfileName(name) { return "", fmt.Errorf("active profile file contains an invalid profile name") }
	path, err := profileDir(name)
	if err != nil { return "", err }
	info, err := os.Stat(path)
	if err != nil { return "", fmt.Errorf("active profile %q is unavailable: %w", name, err) }
	if !info.IsDir() { return "", fmt.Errorf("active profile %q is not a directory", name) }
	return name, nil
}

func tokenPathForProfile(name string) (string, error) {
	if name == defaultProfileName {
		dir, err := configDir()
		if err != nil { return "", err }
		return filepath.Join(dir, "token.json"), nil
	}
	dir, err := profileDir(name)
	if err != nil { return "", err }
	info, err := os.Stat(dir)
	if err != nil { return "", fmt.Errorf("profile %q does not exist", name) }
	if !info.IsDir() { return "", fmt.Errorf("profile %q is not a directory", name) }
	return filepath.Join(dir, "token.json"), nil
}

// CreateProfile creates an empty named profile. Google app credentials remain
// shared; each profile receives a separate token file after its own login.
func CreateProfile(name string) error {
	if !validProfileName(name) {
		return fmt.Errorf("invalid profile name %q (use 1–32 letters/digits, with optional - or _ after the first character; default is reserved)", name)
	}
	root, err := profilesDir()
	if err != nil { return err }
	if err := os.MkdirAll(root, 0o700); err != nil { return err }
	if err := os.Chmod(root, 0o700); err != nil { return err }
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		if os.IsExist(err) { return fmt.Errorf("profile %q already exists", name) }
		return fmt.Errorf("create profile %q: %w", name, err)
	}
	return nil
}

// SelectProfile persists the active profile name without storing any tokens in
// the selection file. The default profile is the pre-existing token.json.
func SelectProfile(name string) error {
	if name != defaultProfileName && !validProfileName(name) {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if name != defaultProfileName {
		path, err := profileDir(name)
		if err != nil { return err }
		info, err := os.Stat(path)
		if err != nil { return fmt.Errorf("profile %q does not exist", name) }
		if !info.IsDir() { return fmt.Errorf("profile %q is not a directory", name) }
	}
	dir, err := configDir()
	if err != nil { return err }
	return writePrivateAtomic(filepath.Join(dir, "active_profile"), []byte(name+"\n"))
}

// RemoveProfile deletes a named profile and its token. The legacy default
// profile cannot be removed; deleting the active profile first switches to
// default so the app never remains pointed at a missing token directory.
func RemoveProfile(name string) error {
	if name == defaultProfileName { return fmt.Errorf("the default profile cannot be removed") }
	if !validProfileName(name) { return fmt.Errorf("invalid profile name %q", name) }
	path, err := profileDir(name)
	if err != nil { return err }
	info, err := os.Stat(path)
	if os.IsNotExist(err) { return fmt.Errorf("profile %q does not exist", name) }
	if err != nil { return err }
	if !info.IsDir() { return fmt.Errorf("profile %q is not a directory", name) }
	active, err := ActiveProfile()
	if err != nil { return err }
	if active == name {
		if err := SelectProfile(defaultProfileName); err != nil { return fmt.Errorf("switch to default profile before removal: %w", err) }
	}
	return os.RemoveAll(path)
}

// ListProfiles returns profile names and token presence only; it never reads
// or returns token contents.
func ListProfiles() ([]ProfileInfo, error) {
	active, err := ActiveProfile()
	if err != nil { return nil, err }
	profiles := []ProfileInfo{{Name: defaultProfileName, Active: active == defaultProfileName}}
	defaultToken, err := tokenPathForProfile(defaultProfileName)
	if err != nil { return nil, err }
	if _, err := os.Stat(defaultToken); err == nil { profiles[0].HasToken = true } else if !os.IsNotExist(err) { return nil, err }

	root, err := profilesDir()
	if err != nil { return nil, err }
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) { return profiles, nil }
	if err != nil { return nil, err }
	for _, entry := range entries {
		if !entry.IsDir() || !validProfileName(entry.Name()) { continue }
		path := filepath.Join(root, entry.Name(), "token.json")
		_, statErr := os.Stat(path)
		hasToken := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) { return nil, statErr }
		profiles = append(profiles, ProfileInfo{Name: entry.Name(), Active: active == entry.Name(), HasToken: hasToken})
	}
	sort.Slice(profiles[1:], func(i, j int) bool { return profiles[i+1].Name < profiles[j+1].Name })
	return profiles, nil
}
