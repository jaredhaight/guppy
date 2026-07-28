package repository

import (
	"fmt"
	"io"
)

// Guppy fetches from a feed it does not control, so every read from the
// network is bounded. Without a bound, a hostile or broken feed can stream
// forever and fill the disk before anything is verified.
const (
	// MaxFeedBytes caps a releases.json / GitHub API response. Release
	// indexes are kilobytes; megabytes is already absurd.
	MaxFeedBytes = 8 << 20 // 8 MiB

	// MaxDownloadBytes caps a single downloaded artifact.
	MaxDownloadBytes = 1 << 30 // 1 GiB
)

// copyLimited copies src into dst, refusing to write more than limit bytes.
//
// It reads one byte past the limit to tell "exactly at the limit" from
// "truncated here", so an oversized artifact is an error rather than a
// silently short file that then fails checksum verification with a confusing
// message.
func copyLimited(dst io.Writer, src io.Reader, limit int64, what string) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("%s exceeds the maximum size of %d bytes", what, limit)
	}
	return n, nil
}
