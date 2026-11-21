package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/store"
)

const backupSchemaVersion = 1

// backupBundle is a portable JSON snapshot of local settings and library
// data. OAuth tokens, client secrets, cookies and profiles are deliberately
// excluded: they live in separate owner-only files and must not be copied
// into a file that users routinely move to cloud storage.
type backupBundle struct {
	SchemaVersion    int             `json:"schema_version"`
	CreatedAt        time.Time       `json:"created_at"`
	Config           json.RawMessage `json:"config,omitempty"`
	Store            json.RawMessage `json:"store,omitempty"`
	DownloadQueue    json.RawMessage `json:"download_queue,omitempty"`
	ProfilesExcluded bool            `json:"profiles_excluded"`
}

func downloadQueuePath() string {
	dir := os.Getenv("YTUI_DOWNLOAD_DIR")
	if dir == "" {
		dir = config.Load().DownloadDir
	}
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Downloads", "ytui")
	}
	return filepath.Join(dir, ".ytui-queue.json")
}

func readOptionalFile(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func runBackupCommand(args []string, stdout, stderr io.Writer) error {
	var yes, noConfig, noStore, noQueue bool
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--yes" || arg == "-y":
			yes = true
		case arg == "--no-config":
			noConfig = true
		case arg == "--no-store":
			noStore = true
		case arg == "--no-queue":
			noQueue = true
		case strings.HasPrefix(arg, "-"):
			return &exitError{code: 2, err: fmt.Errorf("unknown backup option %q", arg)}
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return &exitError{code: 2, err: fmt.Errorf("usage: ytui backup PATH [--yes]")}
	}
	destination := paths[0]
	if !yes {
		if _, err := os.Stat(destination); err == nil {
			return &exitError{code: 2, err: fmt.Errorf("%s already exists (use --yes to overwrite)", destination)}
		}
	}
	var cfgRaw, dataRaw, queueRaw json.RawMessage
	var err error
	if !noConfig {
		cfgRaw, err = readOptionalFile(config.Path())
		if err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	if !noStore {
		dataRaw, err = readOptionalFile(mustStorePath())
		if err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	if !noQueue {
		queueRaw, err = readOptionalFile(downloadQueuePath())
		if err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	bundle := backupBundle{
		SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC(),
		Config: cfgRaw, Store: dataRaw, DownloadQueue: queueRaw, ProfilesExcluded: true,
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return &exitError{code: 1, err: err}
	}
	if err := atomicWriteFile(destination, data, 0o600); err != nil {
		return &exitError{code: 1, err: err}
	}
	fmt.Fprintf(stdout, "backup written to %s (credentials and profiles excluded)\n", destination)
	return nil
}

func mustStorePath() string {
	p, err := store.Path()
	if err != nil {
		return ""
	}
	return p
}

func runRestoreCommand(args []string, stdout, stderr io.Writer) error {
	var yes bool
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--yes" || arg == "-y":
			yes = true
		case strings.HasPrefix(arg, "-"):
			return &exitError{code: 2, err: fmt.Errorf("unknown restore option %q", arg)}
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return &exitError{code: 2, err: fmt.Errorf("usage: ytui restore PATH [--yes]")}
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		return &exitError{code: 1, err: err}
	}
	var bundle backupBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return &exitError{code: 1, err: fmt.Errorf("invalid backup file: %w", err)}
	}
	if bundle.SchemaVersion < 1 || bundle.SchemaVersion > backupSchemaVersion {
		return &exitError{code: 1, err: fmt.Errorf("unsupported backup schema version %d", bundle.SchemaVersion)}
	}
	var cfg config.Config
	if len(bundle.Config) > 0 {
		if err := json.Unmarshal(bundle.Config, &cfg); err != nil {
			return &exitError{code: 1, err: fmt.Errorf("invalid config in backup: %w", err)}
		}
		if err := config.Validate(cfg); err != nil {
			return &exitError{code: 1, err: fmt.Errorf("invalid config in backup: %w", err)}
		}
	}
	if len(bundle.Store) > 0 {
		var check map[string]json.RawMessage
		if err := json.Unmarshal(bundle.Store, &check); err != nil {
			return &exitError{code: 1, err: fmt.Errorf("invalid library data in backup: %w", err)}
		}
	}
	if len(bundle.DownloadQueue) > 0 {
		var check []json.RawMessage
		if err := json.Unmarshal(bundle.DownloadQueue, &check); err != nil {
			return &exitError{code: 1, err: fmt.Errorf("invalid download queue in backup: %w", err)}
		}
	}
	if !yes {
		fmt.Fprintln(stderr, "Restore replaces local config, library data and download queue. Press y to continue.")
		var answer string
		if _, err := fmt.Fscan(os.Stdin, &answer); err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
			return &exitError{code: 1, err: fmt.Errorf("restore canceled")}
		}
	}
	if len(bundle.Config) > 0 {
		if err := atomicWriteFile(config.Path(), bundle.Config, 0o600); err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	if len(bundle.Store) > 0 {
		path := mustStorePath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return &exitError{code: 1, err: err}
		}
		if err := atomicWriteFile(path, bundle.Store, 0o600); err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	if len(bundle.DownloadQueue) > 0 {
		path := downloadQueuePath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return &exitError{code: 1, err: err}
		}
		if err := atomicWriteFile(path, bundle.DownloadQueue, 0o600); err != nil {
			return &exitError{code: 1, err: err}
		}
	}
	fmt.Fprintf(stdout, "restored from %s (credentials and profiles were not included)\n", paths[0])
	return nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ytui-backup-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
