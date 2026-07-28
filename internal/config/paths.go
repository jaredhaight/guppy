package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Environment variables that override the default locations. They exist mainly
// so tests (and anyone running multiple guppy roots) don't touch the real home
// directory: os.UserConfigDir ignores XDG_CONFIG_HOME on macOS and Windows, so
// setting that alone isn't enough.
const (
	EnvConfigDir = "GUPPY_CONFIG_DIR"
	EnvDataDir   = "GUPPY_DATA_DIR"
)

// ConfigDir returns the directory holding guppy's app configuration.
//
//	Linux    ~/.config/guppy
//	macOS    ~/Library/Application Support/guppy
//	Windows  %AppData%\guppy
func ConfigDir() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error locating user config directory: %w", err)
	}
	return filepath.Join(base, "guppy"), nil
}

// DataDir returns the directory holding the binaries and extracted archives
// guppy manages.
//
//	Linux    ~/.local/share/guppy
//	macOS    ~/Library/Application Support/guppy
//	Windows  %LocalAppData%\guppy
//
// ponytail: the standard library has UserConfigDir and UserCacheDir but no
// UserDataDir, so the platform split lives here. On macOS and Windows the
// config directory is already the right home for application data; only the
// XDG platforms separate the two.
func DataDir() (string, error) {
	if dir := os.Getenv(EnvDataDir); dir != "" {
		return dir, nil
	}

	switch runtime.GOOS {
	case "darwin":
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("error locating user data directory: %w", err)
		}
		return filepath.Join(base, "guppy"), nil

	case "windows":
		// %LocalAppData% rather than the roaming %AppData% os.UserConfigDir
		// returns: binaries are machine-specific and shouldn't roam.
		if base := os.Getenv("LocalAppData"); base != "" {
			return filepath.Join(base, "guppy"), nil
		}
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("error locating user data directory: %w", err)
		}
		return filepath.Join(base, "guppy"), nil

	default:
		if base := os.Getenv("XDG_DATA_HOME"); base != "" {
			return filepath.Join(base, "guppy"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error locating user data directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "guppy"), nil
	}
}

// AppsDir returns the directory holding per-app config files.
func AppsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "apps"), nil
}

// BinDir returns the directory guppy links installed binaries into. This is
// the directory users are expected to add to their PATH.
func BinDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin"), nil
}

// InstallDir returns the directory an app's files are installed into.
func InstallDir(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "apps", name), nil
}

// DownloadDir returns the directory downloads are staged in before being
// applied. Contents are transient.
func DownloadDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "downloads"), nil
}
