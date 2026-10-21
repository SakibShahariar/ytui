// Package library discovers media files saved in ytui's download directory.
package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var mediaExtensions = map[string]struct{}{
	".aac": {}, ".avi": {}, ".flac": {}, ".m4a": {}, ".m4v": {},
	".mkv": {}, ".mov": {}, ".mp3": {}, ".mp4": {}, ".oga": {},
	".ogg": {}, ".opus": {}, ".wav": {}, ".webm": {},
}

// File describes a playable media file in the download directory.
type File struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// Scan returns supported media files directly inside dir, newest first.
// Directories, partial downloads, and unknown file types are ignored.
func Scan(dir string) ([]File, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("download directory is empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, ok := mediaExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue // a file may disappear while the directory is being scanned
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, File{
			Path:    filepath.Join(dir, entry.Name()),
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Name < files[j].Name
		}
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files, nil
}

// Delete removes a supported media file directly inside dir. Revalidate the
// path instead of trusting a caller-provided File: this function is exported
// and must not turn an accidental or malicious path into arbitrary file deletion.
func Delete(dir string, file File) error {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(file.Path) == "" {
		return fmt.Errorf("media directory and file path are required")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve media directory: %w", err)
	}
	absPath, err := filepath.Abs(file.Path)
	if err != nil {
		return fmt.Errorf("resolve media file: %w", err)
	}
	if filepath.Dir(absPath) != absDir {
		return fmt.Errorf("refusing to delete a file outside the media directory")
	}
	if strings.HasPrefix(filepath.Base(absPath), ".") {
		return fmt.Errorf("refusing to delete a hidden file")
	}
	if _, ok := mediaExtensions[strings.ToLower(filepath.Ext(absPath))]; !ok {
		return fmt.Errorf("refusing to delete an unsupported media file")
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to delete a non-regular file")
	}
	return os.Remove(absPath)
}
