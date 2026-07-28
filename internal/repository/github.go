package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaredhaight/guppy/internal/version"
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
func (g *GitHubRepository) debugLog(format string, args ...any) {
	if g.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// setAuth attaches the token, if there is one.
//
// The token is never logged. Debug output routinely ends up in bug reports and
// CI logs, and whether authentication was attached is the only part of this
// that helps anyone debug a 401.
func (g *GitHubRepository) setAuth(req *http.Request) {
	if g.Token == "" {
		return
	}
	req.Header.Set("Authorization", "token "+g.Token)
	g.debugLog("Authorization header set (token redacted)")
}

// apiError turns a non-200 into something the user can act on. The raw body
// for a rate-limit response is a wall of JSON that buries the one fact that
// matters: wait, or authenticate.
func (g *GitHubRepository) apiError(resp *http.Response) error {
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			msg := "GitHub API rate limit exceeded"
			if reset := resetTime(resp.Header.Get("X-RateLimit-Reset")); !reset.IsZero() {
				msg += fmt.Sprintf("; it resets at %s", reset.Local().Format("15:04:05"))
			}
			if g.Token == "" {
				msg += ". Unauthenticated requests are limited to 60/hour — set GH_TOKEN or GITHUB_TOKEN to raise it"
			}
			return fmt.Errorf("%s", msg)
		}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// resetTime parses the X-RateLimit-Reset epoch header, returning the zero time
// if it is absent or malformed.
func resetTime(header string) time.Time {
	seconds, err := strconv.ParseInt(header, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// githubAsset represents a single downloadable file attached to a release
type githubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"` // SHA256 checksum in format "sha256:hexvalue"
}

// githubRelease represents a GitHub release API response
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// GetLatestRelease returns the latest release from GitHub
func (g *GitHubRepository) GetLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", g.Owner, g.Repo)
	g.debugLog("Fetching latest release from URL: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "guppy-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	g.setAuth(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, g.apiError(resp)
	}

	var ghRelease githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxFeedBytes)).Decode(&ghRelease); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return g.convertGitHubRelease(&ghRelease)
}

// CompareVersions compares current version with latest
func (g *GitHubRepository) CompareVersions(current, latest string) (bool, error) {
	return version.IsNewer(latest, current)
}

// Download downloads a release to the specified destination
func (g *GitHubRepository) Download(ctx context.Context, release *Release, dest string) error {
	if release.DownloadURL == "" {
		return fmt.Errorf("no download URL in release")
	}

	// Check if we're using the GitHub Asset API
	isAssetAPI := strings.Contains(release.DownloadURL, "/releases/assets/")
	if isAssetAPI {
		g.debugLog("Using GitHub Asset API to download asset ID %d", release.AssetID)
	}
	g.debugLog("Downloading from URL: %s to %s", release.DownloadURL, dest)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("error creating download request: %w", err)
	}

	// Set required headers for GitHub asset downloads
	req.Header.Set("User-Agent", "guppy-updater")
	req.Header.Set("Accept", "application/octet-stream")

	g.setAuth(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = out.Close() }()

	// Copy the content, bounded: the server decides how much it sends.
	if _, err := copyLimited(out, resp.Body, MaxDownloadBytes, "download"); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
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
	var downloadURL, fileName, checksum string
	var assetID int64
	if g.AssetName != "" {
		g.debugLog("Looking for specific asset: %s", g.AssetName)

		asset, err := g.selectAsset(ghRelease.Assets)
		if err != nil {
			return nil, err
		}

		downloadURL = asset.BrowserDownloadURL
		fileName = asset.Name
		assetID = asset.ID
		checksum = parseDigest(asset.Digest)
		g.debugLog("Found matching asset: %s (ID: %d, Checksum: %s)", fileName, assetID, checksum)
	} else {
		// Use the first asset
		downloadURL = ghRelease.Assets[0].BrowserDownloadURL
		fileName = ghRelease.Assets[0].Name
		assetID = ghRelease.Assets[0].ID
		checksum = parseDigest(ghRelease.Assets[0].Digest)
		g.debugLog("Using first asset: %s (ID: %d, Checksum: %s)", fileName, assetID, checksum)
	}

	// If we have a token, use the GitHub Asset API URL instead
	if g.Token != "" && assetID != 0 {
		downloadURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", g.Owner, g.Repo, assetID)
		g.debugLog("Using GitHub Asset API URL: %s", downloadURL)
	}

	if checksum == "" {
		g.debugLog("WARNING: No checksum available for asset %s", fileName)
	}

	// The asset name is whatever the API said it was, and it becomes a path.
	if err := ValidateFileName(fileName); err != nil {
		return nil, err
	}

	return &Release{
		Version:     ghRelease.TagName,
		DownloadURL: downloadURL,
		ReleaseDate: ghRelease.PublishedAt,
		FileName:    fileName,
		AssetID:     assetID,
		Checksum:    checksum,
	}, nil
}

// selectAsset picks the release asset matching AssetName.
//
// An exact filename match wins. Failing that, AssetName is treated as a
// regular expression, because asset names carry the version
// (ripgrep-15.2.0-aarch64-apple-darwin.tar.gz) and a literal name would stop
// matching the moment a new release is published.
func (g *GitHubRepository) selectAsset(assets []githubAsset) (githubAsset, error) {
	for _, asset := range assets {
		if asset.Name == g.AssetName {
			return asset, nil
		}
	}

	pattern, err := regexp.Compile(g.AssetName)
	if err != nil {
		return githubAsset{}, fmt.Errorf("asset %q not found in release, and it is not a valid pattern: %w", g.AssetName, err)
	}

	var matches []githubAsset
	for _, asset := range assets {
		if pattern.MatchString(asset.Name) {
			matches = append(matches, asset)
		}
	}

	switch len(matches) {
	case 0:
		return githubAsset{}, fmt.Errorf("asset %q not found in release (available: %s)",
			g.AssetName, strings.Join(assetNames(assets), ", "))
	case 1:
		return matches[0], nil
	default:
		return githubAsset{}, fmt.Errorf("asset pattern %q matches %d assets: %s",
			g.AssetName, len(matches), strings.Join(assetNames(matches), ", "))
	}
}

func assetNames(assets []githubAsset) []string {
	names := make([]string, len(assets))
	for i, asset := range assets {
		names[i] = asset.Name
	}
	return names
}

// parseDigest normalizes a GitHub asset digest into the "algorithm:hexvalue"
// form Release.Checksum uses. Returns empty string if the digest is empty or
// in a format we don't recognize.
func parseDigest(digest string) string {
	if digest == "" {
		return ""
	}

	// GitHub API returns digest in format "sha256:hexvalue"
	algorithm, value, found := strings.Cut(digest, ":")
	if found && algorithm == "sha256" && value != "" {
		return digest
	}

	return ""
}
