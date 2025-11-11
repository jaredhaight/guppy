package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256 verifies the SHA256 checksum of a file
func VerifySHA256(filePath string, expectedChecksum string) (bool, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("error opening file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Calculate SHA256
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, fmt.Errorf("error calculating checksum: %w", err)
	}

	// Get the checksum as hex string
	actualChecksum := hex.EncodeToString(hash.Sum(nil))

	// Compare checksums (case-insensitive)
	expectedChecksum = strings.ToLower(strings.TrimSpace(expectedChecksum))
	actualChecksum = strings.ToLower(actualChecksum)

	return actualChecksum == expectedChecksum, nil
}

// CalculateSHA256 calculates the SHA256 checksum of a file
func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("error calculating checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
