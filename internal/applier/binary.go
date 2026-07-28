package applier

import (
	"fmt"
	"io"
	"os"
)

// BinaryApplier applies updates by replacing binary files
type BinaryApplier struct{}

// NewBinaryApplier creates a new binary applier
func NewBinaryApplier() *BinaryApplier {
	return &BinaryApplier{}
}

// Apply replaces the target binary with the source binary
func (b *BinaryApplier) Apply(source string, target string) error {
	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	// Get source file info
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("error getting source file info: %w", err)
	}

	// Create temporary target file
	tempTarget := target + ".tmp"
	targetFile, err := os.OpenFile(tempTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("error creating temporary target file: %w", err)
	}

	// Copy source to temp target
	_, err = io.Copy(targetFile, sourceFile)
	_ = targetFile.Close()
	if err != nil {
		_ = os.Remove(tempTarget)
		return fmt.Errorf("error copying file: %w", err)
	}

	// Remove old target if it exists
	if _, err := os.Stat(target); err == nil {
		if err := os.Remove(target); err != nil {
			_ = os.Remove(tempTarget)
			return fmt.Errorf("error removing old target: %w", err)
		}
	}

	// Rename temp to target
	if err := os.Rename(tempTarget, target); err != nil {
		return fmt.Errorf("error renaming temporary file: %w", err)
	}

	// Ensure target is executable (on Unix systems)
	if err := os.Chmod(target, 0755); err != nil {
		return fmt.Errorf("error setting executable permissions: %w", err)
	}

	return nil
}
