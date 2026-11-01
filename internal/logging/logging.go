// Package logging configures ytui's optional diagnostic file logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile("(?i)(access[_-]?token|refresh[_-]?token|client[_-]?secret|client[_-]?id|authorization|password|api[_-]?key|code)([\"']?\\s*[:=]\\s*[\"']?)[^\\s,\"'&}]+") 
var urlPattern = regexp.MustCompile("(?i)(https?://[^\\s?]+\\?[^\\s#]*)")

// Redact removes common credential fields before error text is written to disk.
func Redact(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1$2[REDACTED]")
	return urlPattern.ReplaceAllStringFunc(value, func(raw string) string {
		parts := strings.SplitN(raw, "?", 2)
		if len(parts) != 2 {
			return raw
		}
		query := strings.Split(parts[1], "&")
		for i, item := range query {
			pair := strings.SplitN(item, "=", 2)
			key := strings.ToLower(pair[0])
			if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "key") || key == "code" || key == "authorization" {
				query[i] = pair[0] + "=[REDACTED]"
			}
		}
		return parts[0] + "?" + strings.Join(query, "&")
	})
}

const (
	defaultMaxLogBytes = int64(5 << 20)
	defaultLogBackups  = 3
	maxLogBackups      = 10
)

type rotatingWriter struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxBytes int64
	backups  int
}

func openLogFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func newRotatingWriter(path string, maxBytes int64, backups int) (*rotatingWriter, error) {
	if maxBytes < 1 {
		return nil, fmt.Errorf("log rotation size must be positive")
	}
	if backups < 1 || backups > maxLogBackups {
		return nil, fmt.Errorf("log backup count must be between 1 and %d", maxLogBackups)
	}
	file, err := openLogFile(path)
	if err != nil {
		return nil, err
	}
	writer := &rotatingWriter{file: file, path: path, maxBytes: maxBytes, backups: backups}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.Size() > maxBytes {
		if err := writer.rotateLocked(); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	return writer, nil
}

func logRotationSettings() (int64, int, error) {
	maxBytes := defaultMaxLogBytes
	if raw := strings.TrimSpace(os.Getenv("YTUI_LOG_MAX_BYTES")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1024 {
			return 0, 0, fmt.Errorf("YTUI_LOG_MAX_BYTES must be an integer of at least 1024")
		}
		maxBytes = value
	}
	backups := defaultLogBackups
	if raw := strings.TrimSpace(os.Getenv("YTUI_LOG_BACKUPS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxLogBackups {
			return 0, 0, fmt.Errorf("YTUI_LOG_BACKUPS must be between 1 and %d", maxLogBackups)
		}
		backups = value
	}
	return maxBytes, backups, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := w.file.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > 0 && info.Size()+int64(len(p)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *rotatingWriter) rotateLocked() error {
	if err := w.file.Close(); err != nil {
		w.file = nil
		return fmt.Errorf("close diagnostic log before rotation: %w", err)
	}
	w.file = nil

	oldest := fmt.Sprintf("%s.%d", w.path, w.backups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		w.file, _ = openLogFile(w.path)
		return fmt.Errorf("remove old diagnostic log: %w", err)
	}
	for index := w.backups - 1; index >= 1; index-- {
		source := fmt.Sprintf("%s.%d", w.path, index)
		destination := fmt.Sprintf("%s.%d", w.path, index+1)
		if err := os.Rename(source, destination); err != nil && !os.IsNotExist(err) {
			w.file, _ = openLogFile(w.path)
			return fmt.Errorf("rotate diagnostic log backup: %w", err)
		}
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
		w.file, _ = openLogFile(w.path)
		return fmt.Errorf("rotate diagnostic log: %w", err)
	}
	file, err := openLogFile(w.path)
	if err != nil {
		_ = os.Rename(w.path+".1", w.path)
		w.file, _ = openLogFile(w.path)
		return fmt.Errorf("create new diagnostic log: %w", err)
	}
	w.file = file
	return nil
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// Setup enables structured logging at the requested level. An empty level
// keeps file logging disabled; the returned close function releases the file.
func Setup(level string) (func() error, string, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" || level == "off" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() error { return nil }, "", nil
	}
	var minimum slog.Level
	switch level {
	case "error":
		minimum = slog.LevelError
	case "info":
		minimum = slog.LevelInfo
	case "debug":
		minimum = slog.LevelDebug
	default:
		return nil, "", fmt.Errorf("invalid log level %q (use off, error, info, or debug)", level)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve cache directory: %w", err)
	}
	dir := filepath.Join(cacheDir, "ytui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}
	path := filepath.Join(dir, "ytui.log")
	maxBytes, backups, err := logRotationSettings()
	if err != nil {
		return nil, "", err
	}
	writer, err := newRotatingWriter(path, maxBytes, backups)
	if err != nil {
		return nil, "", fmt.Errorf("open diagnostic log: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: minimum})))
	slog.Info("file logging enabled", "level", level)
	return writer.Close, path, nil
}

// Error records a sanitized operation failure without logging request payloads.
func Error(operation string, err error) {
	if err != nil {
		slog.Error(operation, "error", Redact(err.Error()))
	}
}
