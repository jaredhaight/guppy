package app

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaredhaight/guppy/pkg/applier"
)

// ResolveBin locates a bin entry inside an app's install directory.
//
// An entry is first tried as a path relative to installDir. Failing that, the
// tree is searched for a file with that basename — archives commonly unpack
// into a versioned directory (ripgrep-14.1.1-x86_64-apple-darwin/rg), so a
// literal path in config would break on every release.
func ResolveBin(installDir, entry string) (string, error) {
	if entry == "" {
		return "", fmt.Errorf("empty bin entry")
	}

	installDir = filepath.Clean(installDir)

	// Exact relative path.
	exact := filepath.Join(installDir, entry)
	if applier.Within(installDir, exact) {
		if info, err := os.Stat(exact); err == nil && info.Mode().IsRegular() {
			return exact, nil
		}
	}

	// Otherwise search by basename.
	name := filepath.Base(entry)
	var matches []string
	err := filepath.WalkDir(installDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error searching %s: %w", installDir, err)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no file named %q found in %s", name, installDir)
	case 1:
		return matches[0], nil
	default:
		rel := make([]string, len(matches))
		for i, match := range matches {
			rel[i], _ = filepath.Rel(installDir, match)
		}
		return "", fmt.Errorf("%q is ambiguous in %s, matches: %s (use a relative path)",
			name, installDir, strings.Join(rel, ", "))
	}
}

// LinkBin makes source available as binDir/<name>, replacing anything already
// there. It symlinks where it can and copies where it can't, since symlinks
// need elevation on Windows outside developer mode.
func LinkBin(binDir, source, name string) (string, error) {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("error creating bin directory: %w", err)
	}

	// The archive appliers preserve whatever mode the archive recorded, which
	// isn't always executable.
	if err := os.Chmod(source, 0755); err != nil {
		return "", fmt.Errorf("error making %s executable: %w", source, err)
	}

	link := filepath.Join(binDir, name)
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("error replacing %s: %w", link, err)
	}

	if err := os.Symlink(source, link); err == nil {
		return link, nil
	}

	if err := copyFile(source, link); err != nil {
		return "", fmt.Errorf("error linking %s: %w", link, err)
	}
	return link, nil
}

// UnlinkBins removes every bin entry belonging to an app.
func UnlinkBins(binDir string, names []string) error {
	for _, name := range names {
		link := filepath.Join(binDir, filepath.Base(name))
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing %s: %w", link, err)
		}
	}
	return nil
}

func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
