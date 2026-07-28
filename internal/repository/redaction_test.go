package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The token is a credential. Debug output gets pasted into bug reports and
// scraped out of CI logs, so no code path may ever print it.
const secretToken = "github_pat_11ABCDEFG_supersecretvalue"

// captureStderr runs fn with os.Stderr redirected, and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()

	_ = w.Close()
	return <-done
}

func TestDebugOutputNeverContainsToken(t *testing.T) {
	release := githubRelease{
		TagName: "v1.2.3",
		Assets: []githubAsset{{
			ID:                 456,
			Name:               "tool-linux-amd64",
			BrowserDownloadURL: "https://example.com/tool-linux-amd64",
			Digest:             "sha256:abc123",
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Confirm the token really is being sent — otherwise this test would
		// pass trivially against a client that just dropped it.
		if got := r.Header.Get("Authorization"); got != "token "+secretToken {
			t.Errorf("Authorization header = %q, want the token to be attached", got)
		}
		if strings.HasSuffix(r.URL.Path, "/download") {
			_, _ = w.Write([]byte("payload"))
			return
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "tool")

	output := captureStderr(t, func() {
		repo := NewGitHubRepository("owner", "repo", secretToken)
		repo.SetDebug(true)
		repo.httpClient = server.Client()

		req, err := http.NewRequest("GET", server.URL, nil)
		if err != nil {
			t.Errorf("failed to build request: %v", err)
			return
		}
		repo.setAuth(req)
		resp, err := repo.httpClient.Do(req)
		if err != nil {
			t.Errorf("request failed: %v", err)
			return
		}
		_ = resp.Body.Close()

		if err := repo.Download(context.Background(), &Release{
			Version:     "v1.2.3",
			DownloadURL: server.URL + "/download",
			FileName:    "tool",
		}, dest); err != nil {
			t.Errorf("Download() failed: %v", err)
		}
	})

	if output == "" {
		t.Fatal("expected some debug output, got none — the test would pass vacuously")
	}
	if strings.Contains(output, secretToken) {
		t.Errorf("debug output leaked the token.\ngot:\n%s", output)
	}
	// "token <value>" is the header form; catch a partial leak too.
	if strings.Contains(output, "supersecret") {
		t.Errorf("debug output leaked part of the token.\ngot:\n%s", output)
	}
}
