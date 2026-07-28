package repository

import (
	"context"
	"time"
)

// Release represents a software release
type Release struct {
	Version     string
	DownloadURL string
	Checksum    string
	ReleaseDate time.Time
	FileName    string
	AssetID     int64 // GitHub asset ID (0 if not applicable)
}

// Repository checks for new releases and downloads them
type Repository interface {
	// GetLatestRelease returns the latest release
	GetLatestRelease(ctx context.Context) (*Release, error)

	// CompareVersions compares current version with latest
	// Returns true if latest is newer than current
	CompareVersions(current, latest string) (bool, error)

	// Download downloads a release to the specified destination
	Download(ctx context.Context, release *Release, dest string) error
}
