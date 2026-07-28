package applier

import (
	"fmt"
	"io"
	"os"
)

// Replace deletes path, or moves it aside when it can't be deleted.
//
// This is what makes `guppy update guppy` work on Windows, where the running
// executable can't be deleted but can be renamed. The leftover is cleaned up
// by the next update. On Unix the plain remove succeeds and the fallback never
// runs.
func Replace(path string) error {
	if err := os.Remove(path); err == nil || os.IsNotExist(err) {
		return nil
	}

	aside := path + ".old"
	_ = os.Remove(aside) // stale leftover from a previous self-update
	if err := os.Rename(path, aside); err != nil {
		return fmt.Errorf("error replacing %s: %w", path, err)
	}
	return nil
}

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

	// Clear the old target, which may be the binary running this code
	if err := Replace(target); err != nil {
		_ = os.Remove(tempTarget)
		return err
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
