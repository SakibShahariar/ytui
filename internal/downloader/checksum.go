package downloader

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// Checksum algorithms supported for download verification.
const (
	ChecksumSHA256 = "sha256"
	ChecksumSHA1   = "sha1"
	ChecksumMD5    = "md5"
)

// ParseChecksum validates a hexadecimal digest for one of the supported
// algorithms. The algorithm is supplied separately so callers can expose
// explicit --sha256/--sha1/--md5 flags.
func ParseChecksum(algorithm, encoded string) (string, error) {
	encoded = strings.TrimSpace(strings.ToLower(encoded))
	var size int
	switch strings.ToLower(strings.TrimSpace(algorithm)) {
	case ChecksumSHA256:
		size = sha256.Size
	case ChecksumSHA1:
		size = sha1.Size
	case ChecksumMD5:
		size = md5.Size
	default:
		return "", fmt.Errorf("unsupported checksum algorithm %q (use sha256, sha1, or md5)", algorithm)
	}
	if _, err := hex.DecodeString(encoded); err != nil || len(encoded) != size*2 {
		return "", fmt.Errorf("expected %s checksum to be %d hex characters", algorithm, size*2)
	}
	return encoded, nil
}

func newChecksumHash(algorithm string) hash.Hash {
	switch algorithm {
	case ChecksumSHA256:
		return sha256.New()
	case ChecksumSHA1:
		return sha1.New()
	case ChecksumMD5:
		return md5.New()
	}
	return nil
}

// VerifyFileChecksum returns the file's digest. A mismatch is returned as an
// error containing both values so the UI can report it clearly without
// exposing or deleting the downloaded file.
func VerifyFileChecksum(path, algorithm, expected string) (string, error) {
	h := newChecksumHash(algorithm)
	if h == nil {
		return "", fmt.Errorf("unsupported checksum algorithm %q", algorithm)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum verification: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("read file for checksum verification: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != strings.ToLower(strings.TrimSpace(expected)) {
		return actual, fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return actual, nil
}
