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
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open log file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, "", fmt.Errorf("secure log file: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: minimum})))
	slog.Info("file logging enabled", "level", level)
	return file.Close, path, nil
}

// Error records a sanitized operation failure without logging request payloads.
func Error(operation string, err error) {
	if err != nil {
		slog.Error(operation, "error", Redact(err.Error()))
	}
}
