package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create a test file with known content
	testFilePath := filepath.Join(tempDir, "testfile.txt")
	testContent := []byte("Hello, World!")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// The SHA256 hash of "Hello, World!" is:
	// dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
	validChecksum := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

	tests := []struct {
		name             string
		filePath         string
		expectedChecksum string
		wantValid        bool
		wantErr          bool
	}{
		{
			name:             "valid checksum - lowercase",
			filePath:         testFilePath,
			expectedChecksum: validChecksum,
			wantValid:        true,
			wantErr:          false,
		},
		{
			name:             "valid checksum - uppercase",
			filePath:         testFilePath,
			expectedChecksum: "DFFD6021BB2BD5B0AF676290809EC3A53191DD81C7F70A4B28688A362182986F",
			wantValid:        true,
			wantErr:          false,
		},
		{
			name:             "valid checksum - mixed case",
			filePath:         testFilePath,
			expectedChecksum: "DfFd6021Bb2bD5b0Af676290809eC3a53191dD81c7F70a4B28688a362182986F",
			wantValid:        true,
			wantErr:          false,
		},
		{
			name:             "valid checksum - with whitespace",
			filePath:         testFilePath,
			expectedChecksum: "  dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f  ",
			wantValid:        true,
			wantErr:          false,
		},
		{
			name:             "invalid checksum",
			filePath:         testFilePath,
			expectedChecksum: "0000000000000000000000000000000000000000000000000000000000000000",
			wantValid:        false,
			wantErr:          false,
		},
		{
			name:             "wrong length checksum",
			filePath:         testFilePath,
			expectedChecksum: "abc123",
			wantValid:        false,
			wantErr:          false,
		},
		{
			name:             "file does not exist",
			filePath:         filepath.Join(tempDir, "nonexistent.txt"),
			expectedChecksum: validChecksum,
			wantValid:        false,
			wantErr:          true,
		},
		{
			name:             "empty checksum",
			filePath:         testFilePath,
			expectedChecksum: "",
			wantValid:        false,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := VerifySHA256(tt.filePath, tt.expectedChecksum)

			if (err != nil) != tt.wantErr {
				t.Errorf("VerifySHA256() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if valid != tt.wantValid {
				t.Errorf("VerifySHA256() valid = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestVerifySHA256_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	emptyFilePath := filepath.Join(tempDir, "empty.txt")

	// Create an empty file
	if err := os.WriteFile(emptyFilePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// SHA256 of empty file
	emptyFileChecksum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	valid, err := VerifySHA256(emptyFilePath, emptyFileChecksum)
	if err != nil {
		t.Errorf("VerifySHA256() unexpected error for empty file: %v", err)
	}
	if !valid {
		t.Errorf("VerifySHA256() should validate empty file checksum")
	}
}

func TestVerifySHA256_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	largeFilePath := filepath.Join(tempDir, "large.bin")

	// Create a 1MB file
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFilePath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Calculate the checksum using CalculateSHA256
	expectedChecksum, err := CalculateSHA256(largeFilePath)
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	// Verify it
	valid, err := VerifySHA256(largeFilePath, expectedChecksum)
	if err != nil {
		t.Errorf("VerifySHA256() unexpected error for large file: %v", err)
	}
	if !valid {
		t.Errorf("VerifySHA256() should validate large file checksum")
	}
}

func TestCalculateSHA256(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name             string
		content          []byte
		expectedChecksum string
		wantErr          bool
	}{
		{
			name:             "simple text file",
			content:          []byte("Hello, World!"),
			expectedChecksum: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			wantErr:          false,
		},
		{
			name:             "empty file",
			content:          []byte{},
			expectedChecksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr:          false,
		},
		{
			name:             "binary content",
			content:          []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			expectedChecksum: "c01a8f4c5e7b0e6b4a8e0d7e3f2c1b9a8d7e6f5c4b3a2d1e0f9c8b7a6d5e4f3c",
			wantErr:          false,
		},
		{
			name:             "multiline text",
			content:          []byte("line1\nline2\nline3\n"),
			expectedChecksum: "bb4e2d5d07e23e9e98d65207e5dfb81d80a84c5a1b0f3c0a7eb1e6a0c8d1b5e7",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFilePath := filepath.Join(tempDir, tt.name+".bin")
			if err := os.WriteFile(testFilePath, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Calculate checksum
			checksum, err := CalculateSHA256(testFilePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateSHA256() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the checksum is valid hex string
				if len(checksum) != 64 {
					t.Errorf("CalculateSHA256() returned checksum length = %d, want 64", len(checksum))
				}

				// Verify it matches when we verify it
				valid, err := VerifySHA256(testFilePath, checksum)
				if err != nil {
					t.Errorf("VerifySHA256() error = %v", err)
				}
				if !valid {
					t.Errorf("Calculated checksum %s should validate", checksum)
				}
			}
		})
	}
}

func TestCalculateSHA256_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	nonexistentPath := filepath.Join(tempDir, "does-not-exist.txt")

	_, err := CalculateSHA256(nonexistentPath)
	if err == nil {
		t.Error("CalculateSHA256() expected error for nonexistent file, got nil")
	}
}

func TestCalculateSHA256_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// Try to calculate checksum of a directory
	_, err := CalculateSHA256(tempDir)
	if err == nil {
		t.Error("CalculateSHA256() expected error for directory, got nil")
	}
}

func TestVerifySHA256_SymlinkToFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	testFilePath := filepath.Join(tempDir, "testfile.txt")
	testContent := []byte("Hello, World!")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink to the test file
	symlinkPath := filepath.Join(tempDir, "symlink.txt")
	if err := os.Symlink(testFilePath, symlinkPath); err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}

	validChecksum := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

	// Verify checksum through symlink
	valid, err := VerifySHA256(symlinkPath, validChecksum)
	if err != nil {
		t.Errorf("VerifySHA256() error through symlink: %v", err)
	}
	if !valid {
		t.Error("VerifySHA256() should validate file through symlink")
	}
}

func TestVerify(t *testing.T) {
	content := []byte("test content for checksum verification")
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		checksum  string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid sha256",
			checksum:  "sha256:0bb4f3131cf52feab05638958f23f10539388ba67cd7977f5ffc46add6a3fff5",
			wantValid: true,
		},
		{
			name:      "valid sha1",
			checksum:  "sha1:9972a14ef931c289b5122e6e1b7005e7891f28ff",
			wantValid: true,
		},
		{
			name:      "valid md5",
			checksum:  "md5:d28cd39b02ce37082426395b9385f56e",
			wantValid: true,
		},
		{
			name:      "uppercase hex still matches",
			checksum:  "sha256:0BB4F3131CF52FEAB05638958F23F10539388BA67CD7977F5FFC46ADD6A3FFF5",
			wantValid: true,
		},
		{
			name:      "mismatched hash is invalid, not an error",
			checksum:  "sha256:wronghash",
			wantValid: false,
		},
		{
			name:     "unsupported algorithm",
			checksum: "sha512:abc123",
			wantErr:  true,
		},
		{
			name:     "no algorithm prefix",
			checksum: "sha256abc123",
			wantErr:  true,
		},
		{
			name:     "empty checksum",
			checksum: "",
			wantErr:  true,
		},
		{
			name:     "empty hash value",
			checksum: "sha256:",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := Verify(tmpFile, tt.checksum)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && valid != tt.wantValid {
				t.Errorf("Verify() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

// The HTTP provider emits checksums as "sha256:hex" while the GitHub provider
// used to strip the prefix. Passing the prefixed form to VerifySHA256 compared
// it against a bare digest and always failed, so every HTTP release carrying a
// checksum was unverifiable. Verify handles the prefixed form.
func TestVerifyAcceptsPrefixedChecksumFromHTTPProvider(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "release.bin")
	if err := os.WriteFile(tmpFile, []byte("Hello, World!"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	const digest = "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

	valid, err := Verify(tmpFile, "sha256:"+digest)
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if !valid {
		t.Error("Verify() rejected a correct sha256-prefixed checksum")
	}

	// The old code path: the prefixed value reaching the bare-hex comparison.
	if valid, _ := VerifySHA256(tmpFile, "sha256:"+digest); valid {
		t.Error("VerifySHA256() should not accept a prefixed checksum; use Verify")
	}
}
