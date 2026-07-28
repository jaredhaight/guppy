package applier

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipWith builds a zip whose single entry has the given name, mode and body.
func zipWith(t *testing.T, name string, mode fs.FileMode, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "a.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatalf("CreateHeader() error: %v", err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	return path
}

func extractTo(t *testing.T, archive string) string {
	t.Helper()
	dir := t.TempDir()
	if err := (&ArchiveApplier{ExtractPath: dir}).Apply(archive, filepath.Join(dir, "dummy")); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	return dir
}

// The mode in an archive is chosen by whoever built it, so it is reduced to
// either 0644 or 0755 — never setuid, setgid, sticky or group/world writable.
func TestExtractedModesAreSanitized(t *testing.T) {
	tests := []struct {
		name     string
		mode     fs.FileMode
		wantPerm fs.FileMode
	}{
		{"plain file", 0644, 0644},
		{"executable keeps the bit", 0755, 0755},
		{"setuid executable loses setuid", fs.ModeSetuid | 0755, 0755},
		{"setgid executable loses setgid", fs.ModeSetgid | 0755, 0755},
		{"world-writable file is tightened", 0666, 0644},
		{"world-writable executable is tightened", 0777, 0755},
		{"sticky bit is dropped", fs.ModeSticky | 0644, 0644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := extractTo(t, zipWith(t, "payload", tt.mode, "hello"))

			info, err := os.Stat(filepath.Join(dir, "payload"))
			if err != nil {
				t.Fatalf("Stat() error: %v", err)
			}

			if got := info.Mode().Perm(); got != tt.wantPerm {
				t.Errorf("extracted perm = %04o, want %04o", got, tt.wantPerm)
			}
			// Perm() masks these off, so check the full mode too.
			if bad := info.Mode() & (fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky); bad != 0 {
				t.Errorf("extracted file kept mode bits %v, want none", bad)
			}
		})
	}
}

// A zip entry can name a path outside the destination. It must be refused, not
// clamped, so the caller learns the archive is hostile.
func TestZipEntryEscapeIsRejected(t *testing.T) {
	for _, name := range []string{"../escaped", "../../escaped", "a/../../escaped"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			err := (&ArchiveApplier{ExtractPath: dir}).Apply(
				zipWith(t, name, 0644, "x"), filepath.Join(dir, "dummy"))
			if err == nil {
				t.Fatalf("Apply() accepted an entry named %q", name)
			}
			if !strings.Contains(err.Error(), "illegal file path") {
				t.Errorf("Apply() error = %v, want an illegal-path rejection", err)
			}
		})
	}
}

// An entry whose "../" resolves back inside the destination is legitimate and
// must still extract — the check is containment, not a substring ban.
func TestZipEntryRelativePathInsideIsAllowed(t *testing.T) {
	dir := extractTo(t, zipWith(t, "sub/../payload", 0644, "hello"))

	if _, err := os.Stat(filepath.Join(dir, "payload")); err != nil {
		t.Errorf("an entry resolving back inside the destination was rejected: %v", err)
	}
}

// gzipBomb builds a tar.gz whose single entry expands to size bytes of zeros.
// Highly compressible, so the archive on disk stays small.
func gzipBomb(t *testing.T, size int64) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bomb.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	if err := tw.WriteHeader(&tar.Header{Name: "big", Mode: 0644, Size: size}); err != nil {
		t.Fatalf("WriteHeader() error: %v", err)
	}
	if _, err := io.CopyN(tw, zeroReader{}, size); err != nil {
		t.Fatalf("CopyN() error: %v", err)
	}

	for _, c := range []io.Closer{tw, gz} {
		if err := c.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}
	return path
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// The extractor keeps its own byte count, so an archive that expands past the
// limit is stopped rather than filling the disk.
func TestExtractionIsBounded(t *testing.T) {
	// Exercised against a lowered ceiling by construction: build an entry
	// larger than MaxExtractBytes would be a 2 GiB test. Instead check the
	// budget arithmetic directly, which is the part that can be wrong.
	var sink bytes.Buffer

	if _, err := copyBounded(&sink, strings.NewReader("12345"), 5); err != nil {
		t.Errorf("copyBounded() rejected input that exactly fills the budget: %v", err)
	}
	if _, err := copyBounded(io.Discard, strings.NewReader("123456"), 5); err == nil {
		t.Error("copyBounded() accepted input past the budget")
	}
	if _, err := copyBounded(io.Discard, strings.NewReader("x"), 0); err == nil {
		t.Error("copyBounded() accepted input with no budget left")
	}
	// A later entry must not get a negative budget and wrap into a huge one.
	if _, err := copyBounded(io.Discard, strings.NewReader("x"), -100); err == nil {
		t.Error("copyBounded() accepted input with an overdrawn budget")
	}
}

// The real bomb, end to end, at a size small enough to run quickly: a tar
// entry that fits under the cap still extracts.
func TestModestArchiveStillExtracts(t *testing.T) {
	dir := extractTo(t, gzipBomb(t, 4<<20)) // 4 MiB

	info, err := os.Stat(filepath.Join(dir, "big"))
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Size() != 4<<20 {
		t.Errorf("extracted size = %d, want %d", info.Size(), 4<<20)
	}
}
