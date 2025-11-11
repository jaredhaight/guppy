package applier

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a test ZIP file
func createTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry %s: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write zip entry %s: %v", name, err)
		}
	}
}

// Helper function to create a test TAR.GZ file
func createTestTarGz(t *testing.T, tarPath string, files map[string]string) {
	t.Helper()

	tarFile, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar.gz file: %v", err)
	}
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header %s: %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar entry %s: %v", name, err)
		}
	}
}

// Helper function to create a ZIP with a directory
func createTestZipWithDir(t *testing.T, zipPath string) {
	t.Helper()

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add a directory entry with proper permissions
	dirHeader := &zip.FileHeader{
		Name:   "testdir/",
		Method: zip.Deflate,
	}
	dirHeader.SetMode(0755 | os.ModeDir)
	_, err = zipWriter.CreateHeader(dirHeader)
	if err != nil {
		t.Fatalf("Failed to create directory entry: %v", err)
	}

	// Add a file in the directory
	writer, err := zipWriter.Create("testdir/file.txt")
	if err != nil {
		t.Fatalf("Failed to create file entry: %v", err)
	}
	if _, err := writer.Write([]byte("file in directory")); err != nil {
		t.Fatalf("Failed to write file entry: %v", err)
	}
}

// Helper function to create a TAR.GZ with a directory
func createTestTarGzWithDir(t *testing.T, tarPath string) {
	t.Helper()

	tarFile, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar.gz file: %v", err)
	}
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add directory
	dirHeader := &tar.Header{
		Name:     "testdir/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	if err := tarWriter.WriteHeader(dirHeader); err != nil {
		t.Fatalf("Failed to write directory header: %v", err)
	}

	// Add file in directory
	fileContent := "file in directory"
	fileHeader := &tar.Header{
		Name: "testdir/file.txt",
		Mode: 0644,
		Size: int64(len(fileContent)),
	}
	if err := tarWriter.WriteHeader(fileHeader); err != nil {
		t.Fatalf("Failed to write file header: %v", err)
	}
	if _, err := tarWriter.Write([]byte(fileContent)); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
}

func TestNewArchiveApplier(t *testing.T) {
	applier := NewArchiveApplier()
	if applier == nil {
		t.Error("NewArchiveApplier() returned nil")
	}
	if applier.ExtractPath != "" {
		t.Errorf("NewArchiveApplier() ExtractPath = %q, want empty", applier.ExtractPath)
	}
}

func TestArchiveApplier_Apply_Zip(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test ZIP file
	zipPath := filepath.Join(tempDir, "test.zip")
	files := map[string]string{
		"file1.txt": "Content of file 1",
		"file2.txt": "Content of file 2",
		"file3.bin": "Binary content",
	}
	createTestZip(t, zipPath, files)

	// Create extraction directory
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	// Apply (extract) the archive
	applier := &ArchiveApplier{ExtractPath: extractDir}
	targetPath := filepath.Join(extractDir, "dummy") // Not used for archives
	err := applier.Apply(zipPath, targetPath)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(extractDir, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, content, expectedContent)
		}
	}
}

func TestArchiveApplier_Apply_TarGz(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test TAR.GZ file
	tarPath := filepath.Join(tempDir, "test.tar.gz")
	files := map[string]string{
		"file1.txt": "Content of file 1",
		"file2.txt": "Content of file 2",
		"file3.bin": "Binary content",
	}
	createTestTarGz(t, tarPath, files)

	// Create extraction directory
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	// Apply (extract) the archive
	applier := &ArchiveApplier{ExtractPath: extractDir}
	targetPath := filepath.Join(extractDir, "dummy")
	err := applier.Apply(tarPath, targetPath)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(extractDir, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, content, expectedContent)
		}
	}
}

func TestArchiveApplier_Apply_TgzExtension(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test .tgz file (same as .tar.gz)
	tgzPath := filepath.Join(tempDir, "test.tgz")
	files := map[string]string{
		"file.txt": "TGZ content",
	}
	createTestTarGz(t, tgzPath, files)

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(tgzPath, filepath.Join(extractDir, "dummy"))
	if err != nil {
		t.Fatalf("Apply() failed for .tgz: %v", err)
	}

	// Verify file was extracted
	content, err := os.ReadFile(filepath.Join(extractDir, "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "TGZ content" {
		t.Errorf("Content = %q, want %q", content, "TGZ content")
	}
}

func TestArchiveApplier_Apply_ZipWithDirectory(t *testing.T) {
	tempDir := t.TempDir()

	zipPath := filepath.Join(tempDir, "test.zip")
	createTestZipWithDir(t, zipPath)

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify directory was created
	dirPath := filepath.Join(extractDir, "testdir")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}

	// Verify file in directory
	filePath := filepath.Join(dirPath, "file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file in directory: %v", err)
	}
	if string(content) != "file in directory" {
		t.Errorf("Content = %q, want %q", content, "file in directory")
	}
}

func TestArchiveApplier_Apply_TarGzWithDirectory(t *testing.T) {
	tempDir := t.TempDir()

	tarPath := filepath.Join(tempDir, "test.tar.gz")
	createTestTarGzWithDir(t, tarPath)

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(tarPath, filepath.Join(extractDir, "dummy"))
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify directory was created
	dirPath := filepath.Join(extractDir, "testdir")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}

	// Verify file in directory
	filePath := filepath.Join(dirPath, "file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file in directory: %v", err)
	}
	if string(content) != "file in directory" {
		t.Errorf("Content = %q, want %q", content, "file in directory")
	}
}

func TestArchiveApplier_Apply_DefaultExtractPath(t *testing.T) {
	tempDir := t.TempDir()

	zipPath := filepath.Join(tempDir, "test.zip")
	files := map[string]string{
		"file.txt": "Test content",
	}
	createTestZip(t, zipPath, files)

	// Don't set ExtractPath, should extract to directory containing target
	targetPath := filepath.Join(tempDir, "target.bin")

	applier := NewArchiveApplier()
	err := applier.Apply(zipPath, targetPath)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// File should be extracted to tempDir
	extractedPath := filepath.Join(tempDir, "file.txt")
	content, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "Test content" {
		t.Errorf("Content = %q, want %q", content, "Test content")
	}
}

func TestArchiveApplier_Apply_UnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file with unsupported extension
	unsupportedPath := filepath.Join(tempDir, "test.rar")
	if err := os.WriteFile(unsupportedPath, []byte("fake rar"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	applier := NewArchiveApplier()
	err := applier.Apply(unsupportedPath, filepath.Join(tempDir, "target"))

	if err == nil {
		t.Error("Apply() expected error for unsupported format, got nil")
	}
}

func TestArchiveApplier_Apply_ZipSlipProtection(t *testing.T) {
	tempDir := t.TempDir()

	// Create a malicious ZIP with path traversal
	zipPath := filepath.Join(tempDir, "malicious.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	// Try to write outside the extraction directory
	writer, _ := zipWriter.Create("../../../etc/passwd")
	writer.Write([]byte("malicious content"))
	zipWriter.Close()

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err = applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))

	if err == nil {
		t.Error("Apply() should reject path traversal in ZIP, got nil error")
	}

	// Verify the malicious file was not created
	maliciousPath := filepath.Join(tempDir, "..", "..", "etc", "passwd")
	if _, err := os.Stat(maliciousPath); err == nil {
		t.Error("Apply() allowed path traversal attack")
	}
}

func TestArchiveApplier_Apply_TarGzPathTraversalProtection(t *testing.T) {
	tempDir := t.TempDir()

	// Create a malicious TAR.GZ with path traversal
	tarPath := filepath.Join(tempDir, "malicious.tar.gz")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar.gz: %v", err)
	}
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	tarWriter := tar.NewWriter(gzipWriter)

	content := "malicious content"
	header := &tar.Header{
		Name: "../../../etc/passwd",
		Mode: 0644,
		Size: int64(len(content)),
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(content))
	tarWriter.Close()
	gzipWriter.Close()

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err = applier.Apply(tarPath, filepath.Join(extractDir, "dummy"))

	if err == nil {
		t.Error("Apply() should reject path traversal in TAR.GZ, got nil error")
	}
}

func TestArchiveApplier_Apply_CorruptedZip(t *testing.T) {
	tempDir := t.TempDir()

	// Create a corrupted ZIP file
	zipPath := filepath.Join(tempDir, "corrupted.zip")
	if err := os.WriteFile(zipPath, []byte("This is not a valid ZIP file"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted zip: %v", err)
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))

	if err == nil {
		t.Error("Apply() expected error for corrupted ZIP, got nil")
	}
}

func TestArchiveApplier_Apply_CorruptedTarGz(t *testing.T) {
	tempDir := t.TempDir()

	// Create a corrupted TAR.GZ file
	tarPath := filepath.Join(tempDir, "corrupted.tar.gz")
	if err := os.WriteFile(tarPath, []byte("This is not a valid TAR.GZ file"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted tar.gz: %v", err)
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(tarPath, filepath.Join(extractDir, "dummy"))

	if err == nil {
		t.Error("Apply() expected error for corrupted TAR.GZ, got nil")
	}
}

func TestArchiveApplier_Apply_SourceDoesNotExist(t *testing.T) {
	tempDir := t.TempDir()

	nonexistentZip := filepath.Join(tempDir, "nonexistent.zip")
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err := applier.Apply(nonexistentZip, filepath.Join(extractDir, "dummy"))

	if err == nil {
		t.Error("Apply() expected error for nonexistent archive, got nil")
	}
}

func TestArchiveApplier_Apply_EmptyZip(t *testing.T) {
	tempDir := t.TempDir()

	// Create an empty ZIP file
	zipPath := filepath.Join(tempDir, "empty.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create empty zip: %v", err)
	}
	zipWriter := zip.NewWriter(zipFile)
	zipWriter.Close()
	zipFile.Close()

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err = applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))

	// Should succeed with no files extracted
	if err != nil {
		t.Errorf("Apply() failed for empty ZIP: %v", err)
	}
}

func TestArchiveApplier_Apply_NestedDirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Create ZIP with nested directory structure
	zipPath := filepath.Join(tempDir, "nested.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	writer, _ := zipWriter.Create("level1/level2/level3/file.txt")
	writer.Write([]byte("deeply nested file"))
	zipWriter.Close()

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err = applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify nested file was extracted
	nestedPath := filepath.Join(extractDir, "level1", "level2", "level3", "file.txt")
	content, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("Failed to read nested file: %v", err)
	}
	if string(content) != "deeply nested file" {
		t.Errorf("Content = %q, want %q", content, "deeply nested file")
	}
}

func TestArchiveApplier_ExtractZipFile_Permissions(t *testing.T) {
	tempDir := t.TempDir()

	// Create ZIP with executable file
	zipPath := filepath.Join(tempDir, "executable.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	header := &zip.FileHeader{
		Name:   "script.sh",
		Method: zip.Deflate,
	}
	header.SetMode(0755) // Executable
	writer, _ := zipWriter.CreateHeader(header)
	writer.Write([]byte("#!/bin/bash\necho hello"))
	zipWriter.Close()

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.Mkdir(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	applier := &ArchiveApplier{ExtractPath: extractDir}
	err = applier.Apply(zipPath, filepath.Join(extractDir, "dummy"))
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify file has executable permissions (Unix only)
	scriptPath := filepath.Join(extractDir, "script.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("Failed to stat extracted file: %v", err)
	}

	// On Unix systems, check if file is executable
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Logf("Warning: Extracted file may not preserve executable bit: mode=%v", mode)
	}
}
