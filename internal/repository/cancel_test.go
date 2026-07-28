package repository

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// A download must abort when the context is cancelled. Before contexts were
// plumbed through, Ctrl-C during a transfer did nothing until the 30s client
// timeout expired.
func TestDownloadStopsWhenContextIsCancelled(t *testing.T) {
	started := make(chan struct{})

	// Sends a header, then stalls forever. Cancellation is the only way out.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	// Download uses the release's own URL verbatim, so the test server needs
	// no transport substitution.
	repo := NewGitHubRepository("owner", "repo", "")
	dest := filepath.Join(t.TempDir(), "artifact")

	done := make(chan error, 1)
	go func() {
		done <- repo.Download(ctx, &Release{
			Version:     "1.0.0",
			DownloadURL: server.URL + "/artifact",
			FileName:    "artifact",
		}, dest)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Download() returned nil after cancellation, want an error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Download() error = %v, want it to wrap context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Download() ignored the cancelled context and kept waiting")
	}
}

// Same for the release lookup, which is the first thing every command does.
func TestGetLatestReleaseStopsWhenContextIsCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call

	// The API host is hardcoded, so this one does need the transport swap.
	repo := NewGitHubRepository("owner", "repo", "")
	repo.httpClient = &http.Client{Transport: &mockTransport{serverURL: server.URL}}

	if _, err := repo.GetLatestRelease(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("GetLatestRelease() error = %v, want it to wrap context.Canceled", err)
	}
}
