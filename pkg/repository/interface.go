package repository

import "time"

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
	GetLatestRelease() (*Release, error)

	// GetRelease returns a specific release by version
	GetRelease(version string) (*Release, error)

	// CompareVersions compares current version with latest
	// Returns true if latest is newer than current
	CompareVersions(current, latest string) (bool, error)

	// Download downloads a release to the specified destination
	Download(release *Release, dest string) error
}
