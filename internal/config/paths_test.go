package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvOverrides(t *testing.T) {
	t.Setenv(EnvConfigDir, "/custom/config")
	t.Setenv(EnvDataDir, "/custom/data")

	tests := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{"ConfigDir", ConfigDir, "/custom/config"},
		{"DataDir", DataDir, "/custom/data"},
		{"AppsDir", AppsDir, filepath.Join("/custom/config", "apps")},
		{"BinDir", BinDir, filepath.Join("/custom/data", "bin")},
		{"DownloadDir", DownloadDir, filepath.Join("/custom/data", "downloads")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn()
			if err != nil {
				t.Fatalf("%s() error: %v", tt.name, err)
			}
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}

	got, err := InstallDir("ripgrep")
	if err != nil {
		t.Fatalf("InstallDir() error: %v", err)
	}
	if want := filepath.Join("/custom/data", "apps", "ripgrep"); got != want {
		t.Errorf("InstallDir() = %q, want %q", got, want)
	}
}

// Without overrides the directories must still land somewhere sensible and
// guppy-owned, rather than in the current working directory.
func TestDefaultsAreAbsoluteAndNamespaced(t *testing.T) {
	t.Setenv(EnvConfigDir, "")
	t.Setenv(EnvDataDir, "")

	for _, tt := range []struct {
		name string
		fn   func() (string, error)
	}{
		{"ConfigDir", ConfigDir},
		{"DataDir", DataDir},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn()
			if err != nil {
				t.Skipf("%s() unavailable in this environment: %v", tt.name, err)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("%s() = %q, want an absolute path", tt.name, got)
			}
			if filepath.Base(got) != "guppy" {
				t.Errorf("%s() = %q, want a directory named guppy", tt.name, got)
			}
		})
	}
}

// The bin directory must sit under the data root, not the config root — it
// holds binaries, not configuration.
func TestBinDirIsUnderDataDir(t *testing.T) {
	t.Setenv(EnvConfigDir, "/custom/config")
	t.Setenv(EnvDataDir, "/custom/data")

	bin, err := BinDir()
	if err != nil {
		t.Fatalf("BinDir() error: %v", err)
	}
	if !strings.HasPrefix(bin, "/custom/data") {
		t.Errorf("BinDir() = %q, want it under the data directory", bin)
	}
}
