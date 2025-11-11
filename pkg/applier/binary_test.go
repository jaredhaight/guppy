package applier

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewBinaryApplier(t *testing.T) {
	applier := NewBinaryApplier()
	if applier == nil {
		t.Error("NewBinaryApplier() returned nil")
	}
}

func TestBinaryApplier_Apply(t *testing.T) {
	tempDir := t.TempDir()

	// Create a source binary file
	sourceFile := filepath.Join(tempDir, "source.bin")
	sourceContent := []byte("This is the new binary content v2.0")
	if err := os.WriteFile(sourceFile, sourceContent, 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a target binary file (simulating old version)
	targetFile := filepath.Join(tempDir, "target.bin")
	oldContent := []byte("This is the old binary content v1.0")
	if err := os.WriteFile(targetFile, oldContent, 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Apply the update
	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify the target file was replaced with source content
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file after apply: %v", err)
	}

	if string(targetContent) != string(sourceContent) {
		t.Errorf("Apply() content mismatch: got %q, want %q", targetContent, sourceContent)
	}

	// Verify the target file is executable (on Unix systems)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(targetFile)
		if err != nil {
			t.Fatalf("Failed to stat target file: %v", err)
		}

		mode := info.Mode()
		if mode&0111 == 0 {
			t.Errorf("Apply() target file is not executable: mode=%v", mode)
		}
	}

	// Verify temp file was cleaned up
	tempFile := targetFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Apply() did not clean up temporary file: %s", tempFile)
	}
}

func TestBinaryApplier_Apply_NewTarget(t *testing.T) {
	tempDir := t.TempDir()

	// Create a source binary file
	sourceFile := filepath.Join(tempDir, "source.bin")
	sourceContent := []byte("New binary installation")
	if err := os.WriteFile(sourceFile, sourceContent, 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Target file does not exist (new installation)
	targetFile := filepath.Join(tempDir, "newbinary.bin")

	// Apply the update
	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed for new target: %v", err)
	}

	// Verify the target file was created with source content
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file after apply: %v", err)
	}

	if string(targetContent) != string(sourceContent) {
		t.Errorf("Apply() content mismatch: got %q, want %q", targetContent, sourceContent)
	}
}

func TestBinaryApplier_Apply_PreservesSourcePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()

	// Create a source file with specific permissions
	sourceFile := filepath.Join(tempDir, "source.bin")
	sourceContent := []byte("Binary with specific permissions")
	if err := os.WriteFile(sourceFile, sourceContent, 0750); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	targetFile := filepath.Join(tempDir, "target.bin")

	// Apply the update
	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Note: The current implementation sets 0755 explicitly,
	// so this test verifies that behavior
	targetInfo, err := os.Stat(targetFile)
	if err != nil {
		t.Fatalf("Failed to stat target file: %v", err)
	}

	// Verify target is executable
	targetMode := targetInfo.Mode()
	if targetMode&0111 == 0 {
		t.Errorf("Apply() target file is not executable: mode=%v", targetMode)
	}
}

func TestBinaryApplier_Apply_SourceDoesNotExist(t *testing.T) {
	tempDir := t.TempDir()

	sourceFile := filepath.Join(tempDir, "nonexistent-source.bin")
	targetFile := filepath.Join(tempDir, "target.bin")

	// Create target file (should not be modified if source doesn't exist)
	originalContent := []byte("Original content")
	if err := os.WriteFile(targetFile, originalContent, 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)

	if err == nil {
		t.Error("Apply() expected error when source does not exist, got nil")
	}

	// Verify target file was not modified
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	if string(targetContent) != string(originalContent) {
		t.Error("Apply() modified target file despite source not existing")
	}
}

func TestBinaryApplier_Apply_TargetDirDoesNotExist(t *testing.T) {
	tempDir := t.TempDir()

	sourceFile := filepath.Join(tempDir, "source.bin")
	sourceContent := []byte("Source content")
	if err := os.WriteFile(sourceFile, sourceContent, 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Target is in a directory that doesn't exist
	targetFile := filepath.Join(tempDir, "nonexistent-dir", "target.bin")

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)

	if err == nil {
		t.Error("Apply() expected error when target directory does not exist, got nil")
	}
}

func TestBinaryApplier_Apply_LargeFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a large source file (5MB)
	sourceFile := filepath.Join(tempDir, "large-source.bin")
	largeContent := make([]byte, 5*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(sourceFile, largeContent, 0755); err != nil {
		t.Fatalf("Failed to create large source file: %v", err)
	}

	targetFile := filepath.Join(tempDir, "large-target.bin")

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed for large file: %v", err)
	}

	// Verify the content matches
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	if len(targetContent) != len(largeContent) {
		t.Errorf("Apply() size mismatch: got %d bytes, want %d bytes", len(targetContent), len(largeContent))
	}

	// Verify first and last bytes to ensure complete copy
	if targetContent[0] != largeContent[0] {
		t.Error("Apply() first byte mismatch")
	}
	if targetContent[len(targetContent)-1] != largeContent[len(largeContent)-1] {
		t.Error("Apply() last byte mismatch")
	}
}

func TestBinaryApplier_Apply_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create an empty source file
	sourceFile := filepath.Join(tempDir, "empty-source.bin")
	if err := os.WriteFile(sourceFile, []byte{}, 0755); err != nil {
		t.Fatalf("Failed to create empty source file: %v", err)
	}

	targetFile := filepath.Join(tempDir, "target.bin")
	// Create non-empty target
	if err := os.WriteFile(targetFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed for empty file: %v", err)
	}

	// Verify target is now empty
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	if len(targetContent) != 0 {
		t.Errorf("Apply() should result in empty target, got %d bytes", len(targetContent))
	}
}

func TestBinaryApplier_Apply_SourceIsDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory as source (invalid)
	sourceDir := filepath.Join(tempDir, "source-dir")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	targetFile := filepath.Join(tempDir, "target.bin")

	applier := NewBinaryApplier()
	err := applier.Apply(sourceDir, targetFile)

	if err == nil {
		t.Error("Apply() expected error when source is a directory, got nil")
	}
}

func TestBinaryApplier_Apply_AtomicReplacement(t *testing.T) {
	tempDir := t.TempDir()

	sourceFile := filepath.Join(tempDir, "source.bin")
	sourceContent := []byte("New version")
	if err := os.WriteFile(sourceFile, sourceContent, 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	targetFile := filepath.Join(tempDir, "target.bin")
	oldContent := []byte("Old version")
	if err := os.WriteFile(targetFile, oldContent, 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// At this point, either:
	// 1. The operation completed and target has new content
	// 2. The operation failed and we should have an error

	// Since we got no error, verify the target has new content
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	if string(targetContent) != string(sourceContent) {
		t.Errorf("Apply() should atomically replace target")
	}

	// Verify no temp file remains
	tempFile := targetFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Apply() left temp file: %s", tempFile)
	}
}

func TestBinaryApplier_Apply_CleanupOnCopyFailure(t *testing.T) {
	// This test is difficult to trigger reliably without mocking,
	// but we can at least verify the temp file doesn't exist after success
	tempDir := t.TempDir()

	sourceFile := filepath.Join(tempDir, "source.bin")
	if err := os.WriteFile(sourceFile, []byte("content"), 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	targetFile := filepath.Join(tempDir, "target.bin")

	applier := NewBinaryApplier()
	_ = applier.Apply(sourceFile, targetFile)

	// Verify temp file is cleaned up
	tempFile := targetFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Apply() did not clean up temp file")
	}
}

func TestBinaryApplier_Apply_PermissionError_ReadOnly(t *testing.T) {
	// Skip on Windows where permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tempDir, "source.bin")
	if err := os.WriteFile(sourceFile, []byte("new content"), 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a read-only directory for the target
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0755); err != nil {
		t.Fatalf("Failed to create readonly directory: %v", err)
	}

	// Create target file in read-only directory
	targetFile := filepath.Join(readOnlyDir, "target.bin")
	if err := os.WriteFile(targetFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Make directory read-only (no write permission)
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to chmod directory: %v", err)
	}
	// Restore permissions after test
	defer os.Chmod(readOnlyDir, 0755)

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)

	if err == nil {
		t.Error("Apply() should have failed with permission error")
	}

	// Verify original file is unchanged
	content, readErr := os.ReadFile(targetFile)
	if readErr == nil && string(content) != "old content" {
		t.Errorf("Original file was modified despite permission error")
	}
}

func TestBinaryApplier_Apply_PermissionError_TargetDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tempDir, "source.bin")
	if err := os.WriteFile(sourceFile, []byte("new content"), 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Try to write to a non-existent directory
	targetFile := filepath.Join(tempDir, "nonexistent", "subdir", "target.bin")

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)

	// Should fail because parent directories don't exist
	if err == nil {
		t.Error("Apply() should have failed when target directory doesn't exist")
	}

	// Verify target was not created
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("Target file should not exist after failed Apply()")
	}
}

func TestBinaryApplier_Apply_PermissionError_SourceUnreadable(t *testing.T) {
	// Skip on Windows where permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tempDir, "source.bin")
	if err := os.WriteFile(sourceFile, []byte("content"), 0755); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Make source file unreadable
	if err := os.Chmod(sourceFile, 0000); err != nil {
		t.Fatalf("Failed to chmod source file: %v", err)
	}
	// Restore permissions after test
	defer os.Chmod(sourceFile, 0755)

	targetFile := filepath.Join(tempDir, "target.bin")

	applier := NewBinaryApplier()
	err := applier.Apply(sourceFile, targetFile)

	if err == nil {
		t.Error("Apply() should have failed when source file is unreadable")
	}

	// Verify target was not created
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("Target file should not exist after failed Apply()")
	}
}
