package repository

import (
	"strings"
	"testing"
)

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"plain", "ripgrep-14.1.1-x86_64-apple-darwin.tar.gz", false},
		{"dotted", "app.v2.zip", false},
		{"leading dot", ".hidden", false},

		{"empty", "", true},
		{"dot", ".", true},
		{"parent", "..", true},
		{"traversal", "../../etc/cron.d/evil", true},
		{"absolute", "/etc/passwd", true},
		{"embedded slash", "a/b", true},
		// filepath.Base does not split on backslash off Windows, so a
		// Base-only check would let this through on Linux.
		{"backslash traversal", `..\..\Startup\evil.exe`, true},
		{"null byte", "app\x00.tar.gz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateFileName(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// The GitHub provider takes the asset name straight from the API response and
// turns it into a path, so a hostile or proxied API must not be able to steer
// where guppy writes.
func TestConvertGitHubReleaseRejectsUnsafeAssetName(t *testing.T) {
	repo := NewGitHubRepository("owner", "repo", "")

	_, err := repo.convertGitHubRelease(&githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{{
			ID:                 1,
			Name:               "../../../../.bashrc",
			BrowserDownloadURL: "https://example.com/x",
			Digest:             "sha256:abc",
		}},
	})
	if err == nil {
		t.Fatal("convertGitHubRelease() accepted a traversing asset name")
	}
	if !strings.Contains(err.Error(), "file name") {
		t.Errorf("error = %v, want it to name the bad file name", err)
	}
}

// Same for the HTTP feed, whose file name is the last segment of a URL it
// chooses.
func TestConvertHTTPReleaseRejectsUnsafeFileName(t *testing.T) {
	repo := NewHTTPRepository("https://example.com/releases.json")

	if _, err := repo.convertHTTPRelease(&httpRelease{
		Version: "1.0.0",
		URL:     "https://example.com/..",
		SHA256:  "abc",
	}); err == nil {
		t.Error("convertHTTPRelease() accepted a URL whose last segment is \"..\"")
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https", "https://example.com/releases.json", false},
		{"https with port", "https://example.com:8443/releases.json", false},

		{"http loopback v4", "http://127.0.0.1:8080/releases.json", false},
		{"http loopback name", "http://localhost:3000/releases.json", false},
		{"http loopback v6", "http://[::1]:8080/releases.json", false},

		{"plain http", "http://example.com/releases.json", true},
		{"http on a public ip", "http://93.184.216.34/releases.json", true},
		// 127.0.0.1.evil.com resolves wherever the attacker wants; only a real
		// loopback address earns the carve-out.
		{"loopback-lookalike host", "http://127.0.0.1.evil.com/releases.json", true},

		{"file scheme", "file:///etc/passwd", true},
		{"ftp scheme", "ftp://example.com/x.tar.gz", true},
		{"no scheme", "example.com/releases.json", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
