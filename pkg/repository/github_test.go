package repository

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDigest(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "valid sha256 digest",
			digest:   "sha256:bb3dcd74ea4b8b1c354ef53f0c758a0d75ee8233c2fa34165cdc85bbfc812691",
			expected: "bb3dcd74ea4b8b1c354ef53f0c758a0d75ee8233c2fa34165cdc85bbfc812691",
		},
		{
			name:     "empty digest",
			digest:   "",
			expected: "",
		},
		{
			name:     "invalid format - no colon",
			digest:   "sha256bb3dcd74ea4b8b1c354ef53f0c758a0d75ee8233c2fa34165cdc85bbfc812691",
			expected: "",
		},
		{
			name:     "invalid format - wrong algorithm",
			digest:   "md5:abc123def456",
			expected: "",
		},
		{
			name:     "valid digest with uppercase",
			digest:   "sha256:BB3DCD74EA4B8B1C354EF53F0C758A0D75EE8233C2FA34165CDC85BBFC812691",
			expected: "BB3DCD74EA4B8B1C354EF53F0C758A0D75EE8233C2FA34165CDC85BBFC812691",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDigest(tt.digest)
			if result != tt.expected {
				t.Errorf("parseDigest(%q) = %q, want %q", tt.digest, result, tt.expected)
			}
		})
	}
}

func TestConvertGitHubRelease(t *testing.T) {
	tests := []struct {
		name        string
		ghRelease   *githubRelease
		assetName   string
		wantErr     bool
		wantVersion string
		wantChecksum string
	}{
		{
			name: "release with valid checksum",
			ghRelease: &githubRelease{
				TagName: "v1.0.0",
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 123,
						Name:               "test-binary",
						BrowserDownloadURL: "https://example.com/binary",
						Digest:             "sha256:abc123def456",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v1.0.0",
			wantChecksum: "abc123def456",
		},
		{
			name: "release without checksum",
			ghRelease: &githubRelease{
				TagName: "v1.0.0",
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 123,
						Name:               "test-binary",
						BrowserDownloadURL: "https://example.com/binary",
						Digest:             "",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v1.0.0",
			wantChecksum: "",
		},
		{
			name: "release with no assets",
			ghRelease: &githubRelease{
				TagName: "v1.0.0",
				Assets:  []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{},
			},
			wantErr: true,
		},
		{
			name: "specific asset with checksum",
			ghRelease: &githubRelease{
				TagName: "v1.0.0",
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 123,
						Name:               "wrong-binary",
						BrowserDownloadURL: "https://example.com/wrong",
						Digest:             "sha256:wronghash",
					},
					{
						ID:                 456,
						Name:               "correct-binary",
						BrowserDownloadURL: "https://example.com/correct",
						Digest:             "sha256:correcthash",
					},
				},
			},
			assetName:    "correct-binary",
			wantErr:      false,
			wantVersion:  "v1.0.0",
			wantChecksum: "correcthash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGitHubRepository("owner", "repo", "")
			if tt.assetName != "" {
				g.SetAssetName(tt.assetName)
			}

			release, err := g.convertGitHubRelease(tt.ghRelease)

			if tt.wantErr {
				if err == nil {
					t.Errorf("convertGitHubRelease() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("convertGitHubRelease() unexpected error: %v", err)
				return
			}

			if release.Version != tt.wantVersion {
				t.Errorf("convertGitHubRelease() version = %q, want %q", release.Version, tt.wantVersion)
			}

			if release.Checksum != tt.wantChecksum {
				t.Errorf("convertGitHubRelease() checksum = %q, want %q", release.Checksum, tt.wantChecksum)
			}
		})
	}
}

func TestGitHubRepository_GetLatestRelease(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		assetName      string
		token          string
		wantErr        bool
		wantVersion    string
		wantChecksum   string
		checkAuthHeader bool
	}{
		{
			name:           "successful fetch with checksum",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v1.2.3",
				Name:        "Release v1.2.3",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 456,
						Name:               "test-binary",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.2.3/test-binary",
						Digest:             "sha256:abc123def456",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v1.2.3",
			wantChecksum: "abc123def456",
		},
		{
			name:           "successful fetch without checksum",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v2.0.0",
				Name:        "Release v2.0.0",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 789,
						Name:               "app",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v2.0.0/app",
						Digest:             "",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v2.0.0",
			wantChecksum: "",
		},
		{
			name:           "404 not found",
			responseStatus: http.StatusNotFound,
			responseBody:   map[string]string{"message": "Not Found"},
			wantErr:        true,
		},
		{
			name:           "401 unauthorized",
			responseStatus: http.StatusUnauthorized,
			responseBody:   map[string]string{"message": "Bad credentials"},
			wantErr:        true,
		},
		{
			name:           "403 forbidden - rate limit",
			responseStatus: http.StatusForbidden,
			responseBody:   map[string]string{"message": "API rate limit exceeded"},
			wantErr:        true,
		},
		{
			name:           "429 too many requests",
			responseStatus: http.StatusTooManyRequests,
			responseBody:   map[string]string{"message": "Too many requests"},
			wantErr:        true,
		},
		{
			name:           "500 internal server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   map[string]string{"message": "Internal server error"},
			wantErr:        true,
		},
		{
			name:           "malformed JSON response",
			responseStatus: http.StatusOK,
			responseBody:   "not valid json",
			wantErr:        true,
		},
		{
			name:           "release with no assets",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v1.0.0",
				Name:        "Empty Release",
				PublishedAt: time.Now(),
				Assets:      []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{},
			},
			wantErr: true,
		},
		{
			name:           "specific asset by name",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v1.5.0",
				Name:        "Release v1.5.0",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 100,
						Name:               "app-linux",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.5.0/app-linux",
						Digest:             "sha256:linux123",
					},
					{
						ID:                 101,
						Name:               "app-darwin",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.5.0/app-darwin",
						Digest:             "sha256:darwin456",
					},
				},
			},
			assetName:    "app-darwin",
			wantErr:      false,
			wantVersion:  "v1.5.0",
			wantChecksum: "darwin456",
		},
		{
			name:           "authenticated request with token",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v3.0.0",
				Name:        "Release v3.0.0",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 999,
						Name:               "private-app",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v3.0.0/private-app",
						Digest:             "sha256:private789",
					},
				},
			},
			token:           "ghp_testtoken123",
			wantErr:         false,
			wantVersion:     "v3.0.0",
			wantChecksum:    "private789",
			checkAuthHeader: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check headers
				if r.Header.Get("User-Agent") != "guppy-updater" {
					t.Errorf("Expected User-Agent header to be 'guppy-updater', got %q", r.Header.Get("User-Agent"))
				}
				if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
					t.Errorf("Expected Accept header to be 'application/vnd.github.v3+json', got %q", r.Header.Get("Accept"))
				}

				// Check auth header if token is provided
				if tt.checkAuthHeader && tt.token != "" {
					expectedAuth := fmt.Sprintf("token %s", tt.token)
					if r.Header.Get("Authorization") != expectedAuth {
						t.Errorf("Expected Authorization header to be %q, got %q", expectedAuth, r.Header.Get("Authorization"))
					}
				}

				// Set response status
				w.WriteHeader(tt.responseStatus)

				// Write response body
				if tt.responseStatus == http.StatusOK {
					if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
						t.Fatalf("Failed to encode response: %v", err)
					}
				} else {
					// For error responses
					if str, ok := tt.responseBody.(string); ok {
						_, _ = w.Write([]byte(str))
					} else {
						_ = json.NewEncoder(w).Encode(tt.responseBody)
					}
				}
			}))
			defer server.Close()

			// Create GitHub repository with custom HTTP client
			repo := NewGitHubRepository("owner", "repo", tt.token)
			if tt.assetName != "" {
				repo.SetAssetName(tt.assetName)
			}

			// Override the httpClient to use test server
			repo.httpClient = &http.Client{
				Transport: &mockTransport{
					serverURL: server.URL,
				},
			}

			// Call GetLatestRelease
			release, err := repo.GetLatestRelease()

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetLatestRelease() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetLatestRelease() unexpected error: %v", err)
				return
			}

			// Validate release
			if release.Version != tt.wantVersion {
				t.Errorf("GetLatestRelease() version = %q, want %q", release.Version, tt.wantVersion)
			}

			if release.Checksum != tt.wantChecksum {
				t.Errorf("GetLatestRelease() checksum = %q, want %q", release.Checksum, tt.wantChecksum)
			}

			if release.DownloadURL == "" {
				t.Error("GetLatestRelease() downloadURL is empty")
			}
		})
	}
}

func TestGitHubRepository_GetRelease(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		wantVersion    string
		expectedPath   string
	}{
		{
			name:           "get specific version with v prefix",
			version:        "v1.0.0",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v1.0.0",
				Name:        "Release v1.0.0",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 123,
						Name:               "app",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app",
						Digest:             "sha256:abc123",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v1.0.0",
			expectedPath: "/repos/owner/repo/releases/tags/v1.0.0",
		},
		{
			name:           "get specific version without v prefix",
			version:        "2.5.0",
			responseStatus: http.StatusOK,
			responseBody: githubRelease{
				TagName:     "v2.5.0",
				Name:        "Release v2.5.0",
				PublishedAt: time.Now(),
				Assets: []struct {
					ID                 int64  `json:"id"`
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{
						ID:                 456,
						Name:               "app",
						BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v2.5.0/app",
						Digest:             "sha256:def456",
					},
				},
			},
			wantErr:      false,
			wantVersion:  "v2.5.0",
			expectedPath: "/repos/owner/repo/releases/tags/v2.5.0",
		},
		{
			name:           "version not found",
			version:        "v99.99.99",
			responseStatus: http.StatusNotFound,
			responseBody:   map[string]string{"message": "Not Found"},
			wantErr:        true,
			expectedPath:   "/repos/owner/repo/releases/tags/v99.99.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the correct endpoint was called
				if tt.expectedPath != "" && r.URL.Path != tt.expectedPath {
					t.Errorf("Expected path %q, got %q", tt.expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)

				if tt.responseStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.responseBody)
				} else {
					_ = json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			repo := NewGitHubRepository("owner", "repo", "")
			repo.httpClient = &http.Client{
				Transport: &mockTransport{
					serverURL: server.URL,
				},
			}

			release, err := repo.GetRelease(tt.version)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetRelease() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetRelease() unexpected error: %v", err)
				return
			}

			if release.Version != tt.wantVersion {
				t.Errorf("GetRelease() version = %q, want %q", release.Version, tt.wantVersion)
			}
		})
	}
}

func TestGitHubRepository_Download(t *testing.T) {
	tests := []struct {
		name           string
		release        *Release
		responseStatus int
		responseBody   string
		token          string
		wantErr        bool
		checkAuth      bool
	}{
		{
			name: "successful download",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/test-binary",
				FileName:    "test-binary",
			},
			responseStatus: http.StatusOK,
			responseBody:   "binary content here",
			wantErr:        false,
		},
		{
			name: "successful download with authentication",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "https://api.github.com/repos/owner/repo/releases/assets/123",
				FileName:    "private-binary",
				AssetID:     123,
			},
			responseStatus: http.StatusOK,
			responseBody:   "private binary content",
			token:          "ghp_testtoken",
			wantErr:        false,
			checkAuth:      true,
		},
		{
			name: "download failure - 404",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/missing",
				FileName:    "missing",
			},
			responseStatus: http.StatusNotFound,
			wantErr:        true,
		},
		{
			name: "download failure - 403 forbidden",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/forbidden",
				FileName:    "forbidden",
			},
			responseStatus: http.StatusForbidden,
			wantErr:        true,
		},
		{
			name: "download failure - 500 server error",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/error",
				FileName:    "error",
			},
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name: "missing download URL",
			release: &Release{
				Version:     "v1.0.0",
				DownloadURL: "",
				FileName:    "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip server creation if we expect an error before making the request
			if tt.release.DownloadURL == "" {
				repo := NewGitHubRepository("owner", "repo", tt.token)
				tempDir := t.TempDir()
				dest := filepath.Join(tempDir, "downloaded")

				err := repo.Download(tt.release, dest)
				if !tt.wantErr {
					t.Errorf("Download() expected no error, got %v", err)
				}
				return
			}

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check headers
				if r.Header.Get("User-Agent") != "guppy-updater" {
					t.Errorf("Expected User-Agent header to be 'guppy-updater', got %q", r.Header.Get("User-Agent"))
				}
				if r.Header.Get("Accept") != "application/octet-stream" {
					t.Errorf("Expected Accept header to be 'application/octet-stream', got %q", r.Header.Get("Accept"))
				}

				// Check auth if required
				if tt.checkAuth && tt.token != "" {
					expectedAuth := fmt.Sprintf("token %s", tt.token)
					if r.Header.Get("Authorization") != expectedAuth {
						t.Errorf("Expected Authorization header to be %q, got %q", expectedAuth, r.Header.Get("Authorization"))
					}
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus == http.StatusOK {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			repo := NewGitHubRepository("owner", "repo", tt.token)
			repo.httpClient = &http.Client{
				Transport: &mockTransport{
					serverURL: server.URL,
				},
			}

			// Create temp directory for download
			tempDir := t.TempDir()
			dest := filepath.Join(tempDir, "downloaded")

			err := repo.Download(tt.release, dest)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Download() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Download() unexpected error: %v", err)
				return
			}

			// Verify file was created and has correct content
			content, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("Failed to read downloaded file: %v", err)
			}

			if string(content) != tt.responseBody {
				t.Errorf("Downloaded content = %q, want %q", string(content), tt.responseBody)
			}
		})
	}
}

func TestGitHubRepository_CompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "latest is newer",
			current: "v1.0.0",
			latest:  "v2.0.0",
			want:    true,
		},
		{
			name:    "latest is same",
			current: "v1.5.0",
			latest:  "v1.5.0",
			want:    false,
		},
		{
			name:    "latest is older",
			current: "v3.0.0",
			latest:  "v2.0.0",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewGitHubRepository("owner", "repo", "")
			got, err := repo.CompareVersions(tt.current, tt.latest)
			if err != nil {
				t.Fatalf("CompareVersions() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

// mockTransport is a custom http.RoundTripper that redirects all requests to a test server
type mockTransport struct {
	serverURL string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Preserve the original request path and query
	newURL := m.serverURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}

	// Create new request with modified URL
	newReq, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	newReq.Header = req.Header

	// Use default transport
	return http.DefaultTransport.RoundTrip(newReq)
}
