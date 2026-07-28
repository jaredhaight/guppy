package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

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
// used to strip the prefix. Passing the prefixed form to the old bare-hex
// comparison always failed, so every HTTP release carrying a checksum was
// unverifiable. Verify is now the only entry point and handles the prefix.
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

	// A bare digest is not the documented form and must not be accepted, so
	// the two forms can never be confused again.
	if valid, err := Verify(tmpFile, digest); err == nil && valid {
		t.Error("Verify() accepted a bare digest; the format is \"algorithm:hexvalue\"")
	}
}
