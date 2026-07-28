package applier

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MaxExtractBytes caps the total uncompressed size of an archive.
//
// An archive is attacker-supplied and compresses well: a few hundred KB of
// gzip expands to gigabytes of zeros. Nothing checks the size before
// extraction because nothing knows it, so the extractor keeps its own count
// and stops.
const MaxExtractBytes = 2 << 30 // 2 GiB

// ArchiveApplier applies updates by extracting archives
type ArchiveApplier struct {
	// ExtractPath is the path where the archive will be extracted
	// If empty, extracts to the directory containing the target
	ExtractPath string
}

// Within reports whether path is inside dir. Both are cleaned first, so a
// "../" that resolves back inside is accepted and one that escapes is not.
func Within(dir, path string) bool {
	dir = filepath.Clean(dir)
	path = filepath.Clean(path)
	return path == dir || strings.HasPrefix(path, dir+string(os.PathSeparator))
}

// installMode is the mode an extracted file gets.
//
// The mode inside an archive is chosen by whoever built it. Honoring it
// verbatim lets a release ship a setuid or world-writable file — Go's zip
// reader maps the setuid bit into fs.FileMode and os.OpenFile passes it
// through to the syscall. Whether the file is executable is the only part
// worth carrying over, so that is the only input.
func installMode(executable bool) fs.FileMode {
	if executable {
		return 0755
	}
	return 0644
}

// entryPath resolves an archive entry name against dest, rejecting anything
// that would land outside it.
func entryPath(dest, name string) (string, error) {
	path := filepath.Join(dest, name)
	if !Within(dest, path) {
		return "", fmt.Errorf("illegal file path in archive: %s", name)
	}
	return path, nil
}

// Apply extracts an archive to the target location
func (a *ArchiveApplier) Apply(source string, target string) error {
	// Determine extract path
	extractPath := a.ExtractPath
	if extractPath == "" {
		extractPath = filepath.Dir(target)
	}

	// Determine archive type by extension
	if strings.HasSuffix(source, ".zip") {
		return a.extractZip(source, extractPath)
	} else if strings.HasSuffix(source, ".tar.gz") || strings.HasSuffix(source, ".tgz") {
		return a.extractTarGz(source, extractPath)
	} else {
		return fmt.Errorf("unsupported archive format: %s", source)
	}
}

// extractZip extracts a zip archive
func (a *ArchiveApplier) extractZip(source string, dest string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("error opening zip file: %w", err)
	}
	defer func() { _ = reader.Close() }()

	var written int64
	for _, file := range reader.File {
		path, err := entryPath(dest, file.Name)
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
			continue
		}

		// Symlinks, devices and fifos are skipped, matching the tar path. A
		// zip symlink stores its target as the entry's content, so honoring
		// one would reintroduce the escape the path check above prevents.
		if !file.Mode().IsRegular() {
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("error creating parent directory: %w", err)
		}

		n, err := a.extractZipFile(file, path, MaxExtractBytes-written)
		if err != nil {
			return err
		}
		written += n
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func (a *ArchiveApplier) extractZipFile(file *zip.File, dest string, remaining int64) (int64, error) {
	rc, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("error opening file in archive: %w", err)
	}
	defer func() { _ = rc.Close() }()

	outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, installMode(file.Mode().Perm()&0111 != 0))
	if err != nil {
		return 0, fmt.Errorf("error creating output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	n, err := copyBounded(outFile, rc, remaining)
	if err != nil {
		return n, fmt.Errorf("error extracting %s: %w", file.Name, err)
	}

	return n, nil
}

// extractTarGz extracts a tar.gz archive
func (a *ArchiveApplier) extractTarGz(source string, dest string) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("error opening tar.gz file: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("error creating gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)

	var written int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		path, err := entryPath(dest, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("error creating parent directory: %w", err)
			}

			// header.Mode is an int64 of POSIX bits; only the executable
			// bit is consulted, so it never needs narrowing to a FileMode.
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				installMode(header.Mode&0111 != 0))
			if err != nil {
				return fmt.Errorf("error creating file: %w", err)
			}

			n, err := copyBounded(outFile, tarReader, MaxExtractBytes-written)
			if err != nil {
				_ = outFile.Close()
				return fmt.Errorf("error extracting %s: %w", header.Name, err)
			}
			written += n

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("error closing file: %w", err)
			}
		default:
			// Skip other types (symlinks, devices, etc.)
			continue
		}
	}

	return nil
}

// copyBounded copies src into dst, stopping short of remaining bytes.
//
// It reads one byte past the budget so "filled it exactly" is told apart from
// "there was more", making an over-large archive an error instead of a
// silently truncated install.
func copyBounded(dst io.Writer, src io.Reader, remaining int64) (int64, error) {
	if remaining < 0 {
		remaining = 0
	}
	n, err := io.Copy(dst, io.LimitReader(src, remaining+1))
	if err != nil {
		return n, err
	}
	if n > remaining {
		return n, fmt.Errorf("archive expands past the %d byte limit", MaxExtractBytes)
	}
	return n, nil
}
