package downloader

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseChecksum(t *testing.T) {
	sha256Digest := hex.EncodeToString(sha256.New().Sum(nil))
	sha1Digest := hex.EncodeToString(sha1.New().Sum(nil))
	md5Digest := hex.EncodeToString(md5.New().Sum(nil))
	for _, tt := range []struct{ algorithm, value string }{
		{ChecksumSHA256, strings.ToUpper(sha256Digest)},
		{ChecksumSHA1, sha1Digest},
		{ChecksumMD5, md5Digest},
	} {
		got, err := ParseChecksum(tt.algorithm, tt.value)
		if err != nil || got != strings.ToLower(tt.value) {
			t.Errorf("ParseChecksum(%q, %q) = %q, %v", tt.algorithm, tt.value, got, err)
		}
	}
	for _, tt := range []struct{ algorithm, value string }{
		{ChecksumSHA256, "abc"},
		{ChecksumSHA256, strings.Repeat("z", 64)},
		{ChecksumSHA1, sha256Digest},
		{ChecksumMD5, ""},
		{"sha512", sha256Digest},
	} {
		if _, err := ParseChecksum(tt.algorithm, tt.value); err == nil {
			t.Errorf("ParseChecksum(%q, %q) unexpectedly succeeded", tt.algorithm, tt.value)
		}
	}
}

func TestRunVerifiesChecksumBeforeMarkingDone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	dir := t.TempDir()
	final := filepath.Join(dir, "verified.mp4")
	t.Setenv("YTUI_TEST_FINAL", final)
	binDir := t.TempDir()
	script := `#!/bin/sh
printf '[download] Destination: %s\n' "$YTUI_TEST_FINAL"
printf 'checksum payload' > "$YTUI_TEST_FINAL"
`
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()
	sum := sha256.Sum256([]byte("checksum payload"))
	digest := hex.EncodeToString(sum[:])
	opts := Options{ChecksumAlgorithm: ChecksumSHA256, ExpectedChecksum: digest}
	queued := m.Enqueue("checksum-ok", "Verified", "Channel", "https://example.invalid/v", opts)
	d := waitForTerminalStatus(t, m, queued.VideoID)
	if d.Status != StatusDone || !d.ChecksumVerified || d.ChecksumActual != digest {
		t.Fatalf("status = %v, verified = %v, actual = %q (error: %v)", d.Status, d.ChecksumVerified, d.ChecksumActual, d.Err)
	}
}

func TestRunReportsChecksumMismatchWithoutDeletingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	dir := t.TempDir()
	final := filepath.Join(dir, "bad.mp4")
	t.Setenv("YTUI_TEST_FINAL", final)
	binDir := t.TempDir()
	script := `#!/bin/sh
printf '[download] Destination: %s\n' "$YTUI_TEST_FINAL"
printf 'checksum payload' > "$YTUI_TEST_FINAL"
`
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()
	opts := Options{ChecksumAlgorithm: ChecksumSHA256, ExpectedChecksum: strings.Repeat("0", 64)}
	queued := m.Enqueue("checksum-bad", "Bad", "Channel", "https://example.invalid/v", opts)
	d := waitForTerminalStatus(t, m, queued.VideoID)
	if d.Status != StatusError || d.Err == nil || !strings.Contains(d.Err.Error(), "checksum mismatch") {
		t.Fatalf("status = %v, error = %v; want checksum mismatch", d.Status, d.Err)
	}
	if _, statErr := os.Stat(final); statErr != nil {
		t.Fatalf("mismatched file was deleted: %v", statErr)
	}
}

func TestRunRejectsChecksumWhenFinalPathUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	dir := t.TempDir()
	binDir := t.TempDir()
	// A re-download can finish without a Destination line; verification must
	// not be silently skipped in that case.
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()
	opts := Options{ChecksumAlgorithm: ChecksumSHA256, ExpectedChecksum: strings.Repeat("0", 64)}
	queued := m.Enqueue("checksum-no-path", "No Path", "Channel", "https://example.invalid/v", opts)
	d := waitForTerminalStatus(t, m, queued.VideoID)
	if d.Status != StatusError || d.Err == nil || !strings.Contains(d.Err.Error(), "cannot verify checksum") {
		t.Fatalf("status = %v, error = %v; want checksum verification failure", d.Status, d.Err)
	}
}

func TestVerifyFileChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("checksum payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	actual, err := VerifyFileChecksum(path, ChecksumSHA256, digest)
	if err != nil || actual != digest {
		t.Fatalf("VerifyFileChecksum(match) = %q, %v", actual, err)
	}
	actual, err = VerifyFileChecksum(path, ChecksumSHA256, strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("VerifyFileChecksum(mismatch) = %q, %v; want mismatch error", actual, err)
	}
	if !strings.Contains(err.Error(), digest) {
		t.Fatalf("mismatch error missing actual digest: %v", err)
	}
}
