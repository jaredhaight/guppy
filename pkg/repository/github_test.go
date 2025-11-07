package repository

import (
	"testing"
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
