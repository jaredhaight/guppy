package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jaredhaight/guppy/pkg/version"
)

// GitHubRepository implements Repository for GitHub releases
type GitHubRepository struct {
	Owner      string
	Repo       string
	Token      string // Optional GitHub token for authenticated requests
	AssetName  string // Optional: specific asset name to download
	httpClient *http.Client
	debug      bool
}

// NewGitHubRepository creates a new GitHub repository
func NewGitHubRepository(owner, repo, token string) *GitHubRepository {
	return &GitHubRepository{
		Owner:      owner,
		Repo:       repo,
		Token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetAssetName sets the specific asset name to download
func (g *GitHubRepository) SetAssetName(name string) {
	g.AssetName = name
}

// SetDebug enables or disables debug logging
func (g *GitHubRepository) SetDebug(enabled bool) {
	g.debug = enabled
}

// debugLog prints a debug message if debug mode is enabled
func (g *GitHubRepository) debugLog(format string, args ...interface{}) {
	if g.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// githubRelease represents a GitHub release API response
type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// GetLatestRelease returns the latest release from GitHub
func (g *GitHubRepository) GetLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", g.Owner, g.Repo)
	g.debugLog("Fetching latest release from URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "guppy-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if g.Token != "" {
		authValue := fmt.Sprintf("token %s", g.Token)
		req.Header.Set("Authorization", authValue)
		g.debugLog("Request header set: Authorization: %s", authValue)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ghRelease githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return g.convertGitHubRelease(&ghRelease)
}

// GetRelease returns a specific release by version
func (g *GitHubRepository) GetRelease(version string) (*Release, error) {
	// Ensure version has 'v' prefix for GitHub tags
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", g.Owner, g.Repo, version)
	g.debugLog("Fetching release for version %s from URL: %s", version, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "guppy-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if g.Token != "" {
		authValue := fmt.Sprintf("token %s", g.Token)
		req.Header.Set("Authorization", authValue)
		g.debugLog("Request header set: Authorization: %s", authValue)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ghRelease githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return g.convertGitHubRelease(&ghRelease)
}

// CompareVersions compares current version with latest
func (g *GitHubRepository) CompareVersions(current, latest string) (bool, error) {
	return version.IsNewer(latest, current)
}

// Download downloads a release to the specified destination
func (g *GitHubRepository) Download(release *Release, dest string) error {
	if release.DownloadURL == "" {
		return fmt.Errorf("no download URL in release")
	}

	// Check if we're using the GitHub Asset API
	isAssetAPI := strings.Contains(release.DownloadURL, "/releases/assets/")
	if isAssetAPI {
		g.debugLog("Using GitHub Asset API to download asset ID %d", release.AssetID)
	}
	g.debugLog("Downloading from URL: %s to %s", release.DownloadURL, dest)

	req, err := http.NewRequest("GET", release.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("error creating download request: %w", err)
	}

	// Set required headers for GitHub asset downloads
	req.Header.Set("User-Agent", "guppy-updater")
	req.Header.Set("Accept", "application/octet-stream")

	if g.Token != "" {
		authValue := fmt.Sprintf("token %s", g.Token)
		req.Header.Set("Authorization", authValue)
		g.debugLog("Request header set: Authorization: %s", authValue)
	}

	resp, err := g.httpClient.Do(req)
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

	return nil
}

// convertGitHubRelease converts a GitHub API release to our Release type
func (g *GitHubRepository) convertGitHubRelease(ghRelease *githubRelease) (*Release, error) {
	if len(ghRelease.Assets) == 0 {
		return nil, fmt.Errorf("release has no assets")
	}

	g.debugLog("Release has %d asset(s)", len(ghRelease.Assets))

	// Find the asset to download
	var downloadURL, fileName string
	var assetID int64
	if g.AssetName != "" {
		g.debugLog("Looking for specific asset: %s", g.AssetName)
		// Look for specific asset
		for _, asset := range ghRelease.Assets {
			if asset.Name == g.AssetName {
				downloadURL = asset.BrowserDownloadURL
				fileName = asset.Name
				assetID = asset.ID
				g.debugLog("Found matching asset: %s (ID: %d)", fileName, assetID)
				break
			}
		}
		if downloadURL == "" {
			return nil, fmt.Errorf("asset %s not found in release", g.AssetName)
		}
	} else {
		// Use the first asset
		downloadURL = ghRelease.Assets[0].BrowserDownloadURL
		fileName = ghRelease.Assets[0].Name
		assetID = ghRelease.Assets[0].ID
		g.debugLog("Using first asset: %s (ID: %d)", fileName, assetID)
	}

	// If we have a token, use the GitHub Asset API URL instead
	if g.Token != "" && assetID != 0 {
		downloadURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", g.Owner, g.Repo, assetID)
		g.debugLog("Using GitHub Asset API URL: %s", downloadURL)
	}

	return &Release{
		Version:     ghRelease.TagName,
		DownloadURL: downloadURL,
		ReleaseDate: ghRelease.PublishedAt,
		FileName:    fileName,
		AssetID:     assetID,
	}, nil
}
