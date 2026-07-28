package checksum

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

// Verify checks a file against a checksum in "algorithm:hexvalue" form, e.g.
// "sha256:abc123...". Supported algorithms are sha256, sha1 and md5.
//
// This is the form both repository providers emit, so verification happens in
// one place rather than each provider rolling its own.
func Verify(filePath string, checksum string) (bool, error) {
	algorithm, expected, found := strings.Cut(checksum, ":")
	if !found || algorithm == "" || expected == "" {
		return false, fmt.Errorf("invalid checksum format: %q (want \"algorithm:hexvalue\")", checksum)
	}

	var hasher hash.Hash
	switch algorithm {
	case "sha256":
		hasher = sha256.New()
	case "sha1":
		// Weak, but part of the documented releases.json format.
		hasher = sha1.New()
	case "md5":
		hasher = md5.New()
	default:
		return false, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	actual, err := hashFile(filePath, hasher)
	if err != nil {
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(expected), actual), nil
}

// VerifySHA256 verifies the SHA256 checksum of a file against a bare hex value.
func VerifySHA256(filePath string, expectedChecksum string) (bool, error) {
	actual, err := hashFile(filePath, sha256.New())
	if err != nil {
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(expectedChecksum), actual), nil
}

// CalculateSHA256 calculates the SHA256 checksum of a file
func CalculateSHA256(filePath string) (string, error) {
	return hashFile(filePath, sha256.New())
}

// hashFile streams a file through hasher and returns the hex digest.
func hashFile(filePath string, hasher hash.Hash) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("error calculating checksum: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
