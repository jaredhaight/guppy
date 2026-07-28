package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectChecksum(t *testing.T) {
	tests := []struct {
		name             string
		httpRel          *httpRelease
		expectedChecksum string
		expectedType     string
	}{
		{
			name: "sha256 only",
			httpRel: &httpRelease{
				Version: "1.0.0",
				SHA256:  "abc123",
			},
			expectedChecksum: "sha256:abc123",
			expectedType:     "SHA256",
		},
		{
			name: "sha1 only",
			httpRel: &httpRelease{
				Version: "1.0.0",
				SHA1:    "def456",
			},
			expectedChecksum: "sha1:def456",
			expectedType:     "SHA1",
		},
		{
			name: "md5 only",
			httpRel: &httpRelease{
				Version: "1.0.0",
				MD5:     "789ghi",
			},
			expectedChecksum: "md5:789ghi",
			expectedType:     "MD5",
		},
		{
			name: "all checksums - sha256 preferred",
			httpRel: &httpRelease{
				Version: "1.0.0",
				SHA256:  "abc123",
				SHA1:    "def456",
				MD5:     "789ghi",
			},
			expectedChecksum: "sha256:abc123",
			expectedType:     "SHA256",
		},
		{
			name: "sha1 and md5 - sha1 preferred",
			httpRel: &httpRelease{
				Version: "1.0.0",
				SHA1:    "def456",
				MD5:     "789ghi",
			},
			expectedChecksum: "sha1:def456",
			expectedType:     "SHA1",
		},
		{
			name: "no checksums",
			httpRel: &httpRelease{
				Version: "1.0.0",
			},
			expectedChecksum: "",
			expectedType:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHTTPRepository("http://example.com/releases.json")
			checksum, checksumType := h.selectChecksum(tt.httpRel)

			if checksum != tt.expectedChecksum {
				t.Errorf("selectChecksum() checksum = %q, want %q", checksum, tt.expectedChecksum)
			}

			if checksumType != tt.expectedType {
				t.Errorf("selectChecksum() type = %q, want %q", checksumType, tt.expectedType)
			}
		})
	}
}

func TestGetLatestRelease(t *testing.T) {
	tests := []struct {
		name            string
		releases        []httpRelease
		expectedVersion string
		wantErr         bool
	}{
		{
			name: "single release",
			releases: []httpRelease{
				{Version: "1.0.0", URL: "http://example.com/1.0.0"},
			},
			expectedVersion: "1.0.0",
			wantErr:         false,
		},
		{
			name: "multiple releases - ordered",
			releases: []httpRelease{
				{Version: "1.0.0", URL: "http://example.com/1.0.0"},
				{Version: "2.0.0", URL: "http://example.com/2.0.0"},
				{Version: "3.0.0", URL: "http://example.com/3.0.0"},
			},
			expectedVersion: "3.0.0",
			wantErr:         false,
		},
		{
			name: "multiple releases - unordered",
			releases: []httpRelease{
				{Version: "2.0.0", URL: "http://example.com/2.0.0"},
				{Version: "3.0.0", URL: "http://example.com/3.0.0"},
				{Version: "1.0.0", URL: "http://example.com/1.0.0"},
			},
			expectedVersion: "3.0.0",
			wantErr:         false,
		},
		{
			name: "multiple releases - reverse order",
			releases: []httpRelease{
				{Version: "3.0.0", URL: "http://example.com/3.0.0"},
				{Version: "2.0.0", URL: "http://example.com/2.0.0"},
				{Version: "1.0.0", URL: "http://example.com/1.0.0"},
			},
			expectedVersion: "3.0.0",
			wantErr:         false,
		},
		{
			name: "semantic versions with v prefix",
			releases: []httpRelease{
				{Version: "v1.2.3", URL: "http://example.com/v1.2.3"},
				{Version: "v2.0.0", URL: "http://example.com/v2.0.0"},
				{Version: "v1.9.9", URL: "http://example.com/v1.9.9"},
			},
			expectedVersion: "v2.0.0",
			wantErr:         false,
		},
		{
			name:     "empty releases",
			releases: []httpRelease{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(tt.releases); err != nil {
					t.Errorf("failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			h := NewHTTPRepository(server.URL)
			release, err := h.GetLatestRelease()

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

			if release.Version != tt.expectedVersion {
				t.Errorf("GetLatestRelease() version = %q, want %q", release.Version, tt.expectedVersion)
			}
		})
	}
}

func TestGetRelease(t *testing.T) {
	releases := []httpRelease{
		{Version: "1.0.0", URL: "http://example.com/1.0.0"},
		{Version: "2.0.0", URL: "http://example.com/2.0.0"},
		{Version: "3.0.0", URL: "http://example.com/3.0.0"},
	}

	tests := []struct {
		name            string
		requestVersion  string
		expectedVersion string
		wantErr         bool
	}{
		{
			name:            "exact match",
			requestVersion:  "2.0.0",
			expectedVersion: "2.0.0",
			wantErr:         false,
		},
		{
			name:            "first release",
			requestVersion:  "1.0.0",
			expectedVersion: "1.0.0",
			wantErr:         false,
		},
		{
			name:            "last release",
			requestVersion:  "3.0.0",
			expectedVersion: "3.0.0",
			wantErr:         false,
		},
		{
			name:           "non-existent version",
			requestVersion: "4.0.0",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(releases); err != nil {
					t.Errorf("failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			h := NewHTTPRepository(server.URL)
			release, err := h.GetRelease(tt.requestVersion)

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

			if release.Version != tt.expectedVersion {
				t.Errorf("GetRelease() version = %q, want %q", release.Version, tt.expectedVersion)
			}
		})
	}
}

func TestConvertHTTPRelease(t *testing.T) {
	tests := []struct {
		name             string
		httpRel          *httpRelease
		expectedChecksum string
		expectedFileName string
	}{
		{
			name: "release with sha256",
			httpRel: &httpRelease{
				Version: "1.0.0",
				URL:     "http://example.com/download/app.zip",
				SHA256:  "abc123",
			},
			expectedChecksum: "sha256:abc123",
			expectedFileName: "app.zip",
		},
		{
			name: "release with sha1 only",
			httpRel: &httpRelease{
				Version: "1.0.0",
				URL:     "http://example.com/app.tar.gz",
				SHA1:    "def456",
			},
			expectedChecksum: "sha1:def456",
			expectedFileName: "app.tar.gz",
		},
		{
			name: "release without checksums",
			httpRel: &httpRelease{
				Version: "1.0.0",
				URL:     "http://example.com/binary",
			},
			expectedChecksum: "",
			expectedFileName: "binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHTTPRepository("http://example.com/releases.json")
			release := h.convertHTTPRelease(tt.httpRel)

			if release.Version != tt.httpRel.Version {
				t.Errorf("convertHTTPRelease() version = %q, want %q", release.Version, tt.httpRel.Version)
			}

			if release.DownloadURL != tt.httpRel.URL {
				t.Errorf("convertHTTPRelease() download URL = %q, want %q", release.DownloadURL, tt.httpRel.URL)
			}

			if release.Checksum != tt.expectedChecksum {
				t.Errorf("convertHTTPRelease() checksum = %q, want %q", release.Checksum, tt.expectedChecksum)
			}

			if release.FileName != tt.expectedFileName {
				t.Errorf("convertHTTPRelease() filename = %q, want %q", release.FileName, tt.expectedFileName)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name        string
		fileContent []byte
		checksum    string
		wantErr     bool
	}{
		{
			name:        "successful download with valid checksum",
			fileContent: []byte("test content"),
			checksum:    "sha256:6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
			wantErr:     false,
		},
		{
			name:        "successful download without checksum",
			fileContent: []byte("test content"),
			checksum:    "",
			wantErr:     false,
		},
		{
			// Download no longer verifies; checksum.Verify does, once, in the
			// install pipeline. A bad checksum here is simply carried through.
			name:        "download does not verify the checksum itself",
			fileContent: []byte("test content"),
			checksum:    "sha256:wronghash",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write(tt.fileContent); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			}))
			defer server.Close()

			h := NewHTTPRepository("http://example.com/releases.json")
			tmpDir := t.TempDir()
			destFile := filepath.Join(tmpDir, "downloaded.txt")

			release := &Release{
				DownloadURL: server.URL,
				Checksum:    tt.checksum,
			}

			err := h.Download(release, destFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Download() expected error, got nil")
				}
				// Verify file was removed on checksum failure
				if _, err := os.Stat(destFile); !os.IsNotExist(err) {
					t.Errorf("Download() failed file should have been removed")
				}
				return
			}

			if err != nil {
				t.Errorf("Download() unexpected error: %v", err)
				return
			}

			// Verify file was created
			if _, err := os.Stat(destFile); os.IsNotExist(err) {
				t.Errorf("Download() file was not created")
			}

			// Verify file content
			content, err := os.ReadFile(destFile)
			if err != nil {
				t.Errorf("Download() failed to read downloaded file: %v", err)
			}

			if string(content) != string(tt.fileContent) {
				t.Errorf("Download() content = %q, want %q", string(content), string(tt.fileContent))
			}
		})
	}
}
