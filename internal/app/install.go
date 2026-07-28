// Package app orchestrates checking, downloading and installing a managed app.
package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jaredhaight/guppy/internal/config"
	"github.com/jaredhaight/guppy/internal/hook"
	"github.com/jaredhaight/guppy/pkg/applier"
	"github.com/jaredhaight/guppy/pkg/checksum"
	"github.com/jaredhaight/guppy/pkg/repository"
)

// Installer runs the install pipeline for a single app.
type Installer struct {
	Repo  repository.Repository
	Out   io.Writer
	Debug bool
}

func (i *Installer) out() io.Writer {
	if i.Out != nil {
		return i.Out
	}
	return os.Stdout
}

func (i *Installer) printf(format string, args ...any) {
	fmt.Fprintf(i.out(), format, args...)
}

func (i *Installer) debugf(format string, args ...any) {
	if i.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// NewRepository builds the repository client an app's config describes.
func NewRepository(a *config.App, debug bool) (repository.Repository, error) {
	switch a.Repository.Type {
	case "github":
		repo := repository.NewGitHubRepository(a.Repository.Owner, a.Repository.Repo, a.Repository.Token)
		if a.Repository.AssetName != "" {
			repo.SetAssetName(a.Repository.AssetName)
		}
		repo.SetDebug(debug)
		return repo, nil
	case "http":
		repo := repository.NewHTTPRepository(a.Repository.URL)
		repo.SetDebug(debug)
		return repo, nil
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", a.Repository.Type)
	}
}

// Check reports whether a newer release is available without installing it.
func (i *Installer) Check(a *config.App) error {
	latest, err := i.Repo.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("error getting latest release: %w", err)
	}

	if a.CurrentVersion == "" {
		i.printf("%s: %s available (not installed)\n", a.Name(), latest.Version)
		return nil
	}

	isNewer, err := i.Repo.CompareVersions(a.CurrentVersion, latest.Version)
	if err != nil {
		return fmt.Errorf("error comparing versions: %w", err)
	}

	if isNewer {
		i.printf("%s: 🎉 %s available (current %s)\n", a.Name(), latest.Version, a.CurrentVersion)
	} else {
		i.printf("%s: ✓ up to date (%s)\n", a.Name(), a.CurrentVersion)
	}
	return nil
}

// Install brings an app up to the latest release. It is a no-op when the
// current version is already the latest.
//
// The order is deliberate: download and verify before running any hook, so a
// network failure never leaves a pre_install hook's side effects (a stopped
// service, a drained queue) behind with nothing installed.
func (i *Installer) Install(a *config.App) error {
	latest, err := i.Repo.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("error getting latest release: %w", err)
	}

	if a.CurrentVersion != "" {
		isNewer, err := i.Repo.CompareVersions(a.CurrentVersion, latest.Version)
		if err != nil {
			return fmt.Errorf("error comparing versions: %w", err)
		}
		if !isNewer {
			i.printf("%s: ✓ up to date (%s)\n", a.Name(), a.CurrentVersion)
			return nil
		}
	}

	installDir, err := config.InstallDir(a.Name())
	if err != nil {
		return err
	}
	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	downloadDir, err := config.DownloadDir()
	if err != nil {
		return err
	}

	// Download.
	i.printf("%s: downloading %s...\n", a.Name(), latest.Version)
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("error creating download directory: %w", err)
	}
	downloadPath := filepath.Join(downloadDir, latest.FileName)
	i.debugf("Computed download path: %s", downloadPath)
	if err := i.Repo.Download(latest, downloadPath); err != nil {
		return fmt.Errorf("error downloading release: %w", err)
	}
	defer func() { _ = os.Remove(downloadPath) }()

	// Verify.
	if latest.Checksum != "" {
		valid, err := checksum.Verify(downloadPath, latest.Checksum)
		if err != nil {
			return fmt.Errorf("error verifying checksum: %w", err)
		}
		if !valid {
			_ = os.Remove(downloadPath)
			return fmt.Errorf("checksum verification failed - file may be corrupted")
		}
		i.printf("%s: ✓ checksum verified\n", a.Name())
	} else {
		i.debugf("No checksum available for %s", latest.Version)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("error creating install directory: %w", err)
	}

	runner := &hook.Runner{
		Dir:    installDir,
		Env:    hookEnv(a, latest.Version, installDir, binDir, downloadPath),
		Stdout: i.out(),
	}

	// Hooks that stop services run here, once we know we have a good file.
	if len(a.PreInstall) > 0 {
		i.printf("%s: running pre_install...\n", a.Name())
		if err := runner.Run(a.PreInstall); err != nil {
			return fmt.Errorf("pre_install: %w", err)
		}
	}

	if err := i.apply(a, downloadPath, installDir); err != nil {
		return err
	}

	// Link binaries into the bin directory.
	for _, entry := range a.Binaries() {
		source, err := ResolveBin(installDir, entry)
		if err != nil {
			return fmt.Errorf("error resolving bin entry %q: %w", entry, err)
		}
		link, err := LinkBin(binDir, source, filepath.Base(entry))
		if err != nil {
			return err
		}
		i.printf("%s: linked %s\n", a.Name(), link)
	}

	// Record the new version before post_install, so a failing hook doesn't
	// make guppy re-download an already-installed release on the next run.
	previous := a.CurrentVersion
	a.CurrentVersion = latest.Version
	if err := a.Save(); err != nil {
		i.printf("Warning: could not save updated version to config: %v\n", err)
	}

	i.printf("%s: ✓ installed %s\n", a.Name(), latest.Version)

	if len(a.PostInstall) > 0 {
		i.printf("%s: running post_install...\n", a.Name())
		runner.Env = hookEnv(a, latest.Version, installDir, binDir, downloadPath)
		runner.Env["GUPPY_PREVIOUS_VERSION"] = previous
		if err := runner.Run(a.PostInstall); err != nil {
			return fmt.Errorf("post_install: %w (%s %s is installed; the hook failed, not the install)",
				err, a.Name(), latest.Version)
		}
	}

	return nil
}

// Remove deletes everything guppy created for an app: its bin links, its
// install directory and its config file.
func Remove(a *config.App) error {
	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	if err := UnlinkBins(binDir, a.Binaries()); err != nil {
		return err
	}

	installDir, err := config.InstallDir(a.Name())
	if err != nil {
		return err
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("error removing install directory: %w", err)
	}

	return config.RemoveApp(a.Name())
}

// apply puts the downloaded file into the app's install directory.
//
// Archives are extracted into a sibling directory and swapped in, so a failed
// extraction leaves the working install untouched.
func (i *Installer) apply(a *config.App, downloadPath, installDir string) error {
	switch a.Applier {
	case "binary":
		target := filepath.Join(installDir, filepath.Base(a.Binaries()[0]))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("error creating install directory: %w", err)
		}
		if err := applier.NewBinaryApplier().Apply(downloadPath, target); err != nil {
			return fmt.Errorf("error applying update: %w", err)
		}
		return nil

	case "archive":
		staging := installDir + ".new"
		if err := os.RemoveAll(staging); err != nil {
			return fmt.Errorf("error clearing staging directory: %w", err)
		}
		if err := os.MkdirAll(staging, 0755); err != nil {
			return fmt.Errorf("error creating staging directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(staging) }()

		archive := &applier.ArchiveApplier{ExtractPath: staging}
		if err := archive.Apply(downloadPath, staging); err != nil {
			return fmt.Errorf("error extracting archive: %w", err)
		}

		if err := os.RemoveAll(installDir); err != nil {
			return fmt.Errorf("error clearing install directory: %w", err)
		}
		if err := os.Rename(staging, installDir); err != nil {
			return fmt.Errorf("error moving extracted files into place: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown applier type: %s", a.Applier)
	}
}

// hookEnv builds the environment hook commands see. Release data is passed
// here rather than interpolated into the command text, so a hostile version
// string can't become shell syntax.
func hookEnv(a *config.App, version, installDir, binDir, downloadPath string) map[string]string {
	return map[string]string{
		"GUPPY_APP":              a.Name(),
		"GUPPY_VERSION":          version,
		"GUPPY_PREVIOUS_VERSION": a.CurrentVersion,
		"GUPPY_INSTALL_DIR":      installDir,
		"GUPPY_BIN_DIR":          binDir,
		"GUPPY_DOWNLOAD":         downloadPath,
	}
}
