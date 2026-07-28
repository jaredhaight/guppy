package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes a set of relative paths under root.
func tree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho hi\n"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}
}

func TestResolveBinExactPath(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, "bin/rg", "rg")

	got, err := ResolveBin(dir, "bin/rg")
	if err != nil {
		t.Fatalf("ResolveBin() error: %v", err)
	}
	if want := filepath.Join(dir, "bin", "rg"); got != want {
		t.Errorf("ResolveBin() = %q, want the exact path %q", got, want)
	}
}

// The common case: an archive that unpacks under a versioned directory, so the
// config can't name a stable relative path.
func TestResolveBinFindsNestedBasename(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, "ripgrep-14.1.1-x86_64-apple-darwin/rg",
		"ripgrep-14.1.1-x86_64-apple-darwin/README",
		"ripgrep-14.1.1-x86_64-apple-darwin/doc/rg.1")

	got, err := ResolveBin(dir, "rg")
	if err != nil {
		t.Fatalf("ResolveBin() error: %v", err)
	}
	if want := filepath.Join(dir, "ripgrep-14.1.1-x86_64-apple-darwin", "rg"); got != want {
		t.Errorf("ResolveBin() = %q, want %q", got, want)
	}
}

func TestResolveBinErrors(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		entry   string
		wantErr string
	}{
		{
			name:    "no match",
			files:   []string{"pkg/other"},
			entry:   "rg",
			wantErr: "no file named",
		},
		{
			name:    "ambiguous match",
			files:   []string{"a/rg", "b/rg"},
			entry:   "rg",
			wantErr: "ambiguous",
		},
		{
			name:    "empty entry",
			files:   []string{"rg"},
			entry:   "",
			wantErr: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tree(t, dir, tt.files...)

			_, err := ResolveBin(dir, tt.entry)
			if err == nil {
				t.Fatalf("ResolveBin(%q) succeeded, want error", tt.entry)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ResolveBin() error = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

// An ambiguous entry can be disambiguated by giving the relative path.
func TestResolveBinRelativePathBeatsAmbiguity(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, "a/rg", "b/rg")

	got, err := ResolveBin(dir, "b/rg")
	if err != nil {
		t.Fatalf("ResolveBin() error: %v", err)
	}
	if want := filepath.Join(dir, "b", "rg"); got != want {
		t.Errorf("ResolveBin() = %q, want %q", got, want)
	}
}

// A bin entry must not be able to reach outside the install directory.
func TestResolveBinRejectsEscape(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	tree(t, installDir, "keep")

	outside := filepath.Join(root, "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	// "../secret" must not resolve to the file outside installDir. Falling back
	// to a basename search inside the tree is fine; escaping is not.
	got, err := ResolveBin(installDir, "../secret")
	if err == nil && !strings.HasPrefix(got, installDir) {
		t.Errorf("ResolveBin() = %q, escaped the install directory", got)
	}
}

func TestLinkBinSymlinksAndMakesExecutable(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	binDir := filepath.Join(root, "bin")
	tree(t, installDir, "rg") // written 0644, deliberately not executable

	source := filepath.Join(installDir, "rg")
	link, err := LinkBin(binDir, source, "rg")
	if err != nil {
		t.Fatalf("LinkBin() error: %v", err)
	}

	if want := filepath.Join(binDir, "rg"); link != want {
		t.Errorf("LinkBin() = %q, want %q", link, want)
	}

	// The archive appliers preserve archive modes, so LinkBin has to chmod.
	info, err := os.Stat(source)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("LinkBin() left source mode %v, want it executable", info.Mode().Perm())
	}

	// And the link must reach the real file.
	if _, err := os.Stat(link); err != nil {
		t.Errorf("LinkBin() produced a broken link: %v", err)
	}
}

func TestLinkBinReplacesExisting(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	binDir := filepath.Join(root, "bin")
	tree(t, installDir, "v1/rg", "v2/rg")

	if _, err := LinkBin(binDir, filepath.Join(installDir, "v1", "rg"), "rg"); err != nil {
		t.Fatalf("first LinkBin() error: %v", err)
	}
	link, err := LinkBin(binDir, filepath.Join(installDir, "v2", "rg"), "rg")
	if err != nil {
		t.Fatalf("second LinkBin() error: %v", err)
	}

	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("EvalSymlinks() error: %v", err)
	}
	if !strings.Contains(resolved, "v2") {
		t.Errorf("link resolves to %q, want the v2 binary", resolved)
	}
}

func TestUnlinkBins(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	binDir := filepath.Join(root, "bin")
	tree(t, installDir, "rg", "sub/fd")

	for _, entry := range []string{"rg", "sub/fd"} {
		source := filepath.Join(installDir, entry)
		if _, err := LinkBin(binDir, source, filepath.Base(entry)); err != nil {
			t.Fatalf("LinkBin(%q) error: %v", entry, err)
		}
	}

	if err := UnlinkBins(binDir, []string{"rg", "sub/fd"}); err != nil {
		t.Fatalf("UnlinkBins() error: %v", err)
	}

	for _, name := range []string{"rg", "fd"} {
		if _, err := os.Lstat(filepath.Join(binDir, name)); !os.IsNotExist(err) {
			t.Errorf("UnlinkBins() left %s behind", name)
		}
	}

	// Removing links that are already gone is not an error.
	if err := UnlinkBins(binDir, []string{"rg"}); err != nil {
		t.Errorf("UnlinkBins() on missing links error: %v", err)
	}
}
