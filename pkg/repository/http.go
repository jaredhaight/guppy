package repository

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jaredhaight/guppy/pkg/version"
)

// HTTPRepository implements Repository for HTTP-based releases
type HTTPRepository struct {
	URL        string
	httpClient *http.Client
	debug      bool
}

// NewHTTPRepository creates a new HTTP repository
func NewHTTPRepository(url string) *HTTPRepository {
	return &HTTPRepository{
		URL:        url,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetDebug enables or disables debug logging
func (h *HTTPRepository) SetDebug(enabled bool) {
	h.debug = enabled
}

// debugLog prints a debug message if debug mode is enabled
func (h *HTTPRepository) debugLog(format string, args ...interface{}) {
	if h.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// httpRelease represents a release in the releases.json format
type httpRelease struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	MD5     string `json:"md5"`
	SHA1    string `json:"sha1"`
	SHA256  string `json:"sha256"`
}

// fetchReleases fetches and parses the releases.json file
func (h *HTTPRepository) fetchReleases() ([]httpRelease, error) {
	h.debugLog("Fetching releases from URL: %s", h.URL)

	req, err := http.NewRequest("GET", h.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "guppy-updater")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []httpRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("error decoding releases JSON: %w", err)
	}

	h.debugLog("Fetched %d release(s)", len(releases))
	return releases, nil
}

// GetLatestRelease returns the latest release by comparing all versions
func (h *HTTPRepository) GetLatestRelease() (*Release, error) {
	releases, err := h.fetchReleases()
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	// Find the latest version by comparing all releases
	var latestRelease *httpRelease
	for i := range releases {
		if latestRelease == nil {
			latestRelease = &releases[i]
			continue
		}

		// Compare versions to find the newest
		isNewer, err := version.IsNewer(releases[i].Version, latestRelease.Version)
		if err != nil {
			h.debugLog("Error comparing versions %s and %s: %v", releases[i].Version, latestRelease.Version, err)
			continue
		}

		if isNewer {
			latestRelease = &releases[i]
		}
	}

	if latestRelease == nil {
		return nil, fmt.Errorf("no valid release found")
	}

	h.debugLog("Latest release: %s", latestRelease.Version)
	return h.convertHTTPRelease(latestRelease), nil
}

// GetRelease returns a specific release by version
func (h *HTTPRepository) GetRelease(version string) (*Release, error) {
	releases, err := h.fetchReleases()
	if err != nil {
		return nil, err
	}

	h.debugLog("Looking for release version: %s", version)

	for i := range releases {
		if releases[i].Version == version {
			h.debugLog("Found matching release: %s", version)
			return h.convertHTTPRelease(&releases[i]), nil
		}
	}

	return nil, fmt.Errorf("release version %s not found", version)
}

// CompareVersions compares current version with latest
func (h *HTTPRepository) CompareVersions(current, latest string) (bool, error) {
	return version.IsNewer(latest, current)
}

// Download downloads a release to the specified destination
func (h *HTTPRepository) Download(release *Release, dest string) error {
	if release.DownloadURL == "" {
		return fmt.Errorf("no download URL in release")
	}

	h.debugLog("Downloading from URL: %s to %s", release.DownloadURL, dest)

	req, err := http.NewRequest("GET", release.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("error creating download request: %w", err)
	}

	req.Header.Set("User-Agent", "guppy-updater")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	// Create the destination file
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer out.Close()

	// Copy the content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing to destination: %w", err)
	}

	// Verify checksum if available
	if release.Checksum != "" {
		h.debugLog("Verifying checksum: %s", release.Checksum)
		if err := h.verifyChecksum(dest, release.Checksum); err != nil {
			// Remove the downloaded file if checksum verification fails
			os.Remove(dest)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		h.debugLog("Checksum verification passed")
	} else {
		h.debugLog("WARNING: No checksum available for verification")
	}

	return nil
}

// convertHTTPRelease converts an HTTP release to our Release type
func (h *HTTPRepository) convertHTTPRelease(httpRel *httpRelease) *Release {
	checksum, checksumType := h.selectChecksum(httpRel)
	if checksum != "" {
		h.debugLog("Selected %s checksum: %s", checksumType, checksum)
	} else {
		h.debugLog("WARNING: No checksum available for version %s", httpRel.Version)
	}

	// Extract filename from URL
	fileName := filepath.Base(httpRel.URL)

	return &Release{
		Version:     httpRel.Version,
		DownloadURL: httpRel.URL,
		FileName:    fileName,
		Checksum:    checksum,
		// ReleaseDate is not available in the HTTP format
		ReleaseDate: time.Time{},
		AssetID:     0,
	}
}

// selectChecksum selects the highest priority checksum from available options
// Priority: SHA256 > SHA1 > MD5
func (h *HTTPRepository) selectChecksum(httpRel *httpRelease) (string, string) {
	if httpRel.SHA256 != "" {
		return "sha256:" + httpRel.SHA256, "SHA256"
	}
	if httpRel.SHA1 != "" {
		return "sha1:" + httpRel.SHA1, "SHA1"
	}
	if httpRel.MD5 != "" {
		return "md5:" + httpRel.MD5, "MD5"
	}
	return "", ""
}

// verifyChecksum verifies the downloaded file against the checksum
// Checksum format: "algorithm:hexvalue" (e.g., "sha256:abc123...")
func (h *HTTPRepository) verifyChecksum(filePath, checksum string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Parse the checksum format
	var algorithm, expectedHash string
	for i, c := range checksum {
		if c == ':' {
			algorithm = checksum[:i]
			expectedHash = checksum[i+1:]
			break
		}
	}

	if algorithm == "" || expectedHash == "" {
		return fmt.Errorf("invalid checksum format: %s", checksum)
	}

	// Calculate the hash based on the algorithm
	var actualHash string
	switch algorithm {
	case "sha256":
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return fmt.Errorf("error calculating SHA256: %w", err)
		}
		actualHash = hex.EncodeToString(hasher.Sum(nil))
	case "sha1":
		hasher := sha1.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return fmt.Errorf("error calculating SHA1: %w", err)
		}
		actualHash = hex.EncodeToString(hasher.Sum(nil))
	case "md5":
		hasher := md5.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return fmt.Errorf("error calculating MD5: %w", err)
		}
		actualHash = hex.EncodeToString(hasher.Sum(nil))
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	// Compare the hashes
	if actualHash != expectedHash {
		return fmt.Errorf("%s mismatch: expected %s, got %s", algorithm, expectedHash, actualHash)
	}

	return nil
}
