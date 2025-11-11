package applier

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ArchiveApplier applies updates by extracting archives
type ArchiveApplier struct {
	// ExtractPath is the path where the archive will be extracted
	// If empty, extracts to the directory containing the target
	ExtractPath string
}

// NewArchiveApplier creates a new archive applier
func NewArchiveApplier() *ArchiveApplier {
	return &ArchiveApplier{}
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

	for _, file := range reader.File {
		path := filepath.Join(dest, file.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("error creating parent directory: %w", err)
		}

		// Extract file
		if err := a.extractZipFile(file, path); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func (a *ArchiveApplier) extractZipFile(file *zip.File, dest string) error {
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("error opening file in archive: %w", err)
	}
	defer func() { _ = rc.Close() }()

	outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	_, err = io.Copy(outFile, rc)
	if err != nil {
		return fmt.Errorf("error extracting file: %w", err)
	}

	return nil
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

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		path := filepath.Join(dest, header.Name)

		// Check for path traversal
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("error creating parent directory: %w", err)
			}

			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("error creating file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("error extracting file: %w", err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("error closing file: %w", err)
			}
		default:
			// Skip other types (symlinks, etc.)
			continue
		}
	}

	return nil
}
