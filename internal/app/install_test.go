package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jaredhaight/guppy/internal/config"
	"github.com/jaredhaight/guppy/internal/repository"
	"github.com/jaredhaight/guppy/internal/version"
)

// testRoot points config and data at a temp dir so tests never touch the real
// home directory.
func testRoot(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv(config.EnvConfigDir, filepath.Join(root, "config"))
	t.Setenv(config.EnvDataDir, filepath.Join(root, "data"))
}

// tarGz builds a .tar.gz whose entries are the given name->content pairs.
// Files are given mode 0644 so the tests exercise the chmod in LinkBin.
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar body: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fakeRepo stands in for a provider so the install pipeline can be tested
// without one.
//
// It is a fake rather than a test server because GitHubRepository addresses
// api.github.com unconditionally and keeps its HTTP client unexported, so
// there is nothing this package can point somewhere else. Testing Install
// against a real provider would only be testing the provider twice over; what
// matters here is what Install does with the release it is handed.
type fakeRepo struct {
	release *repository.Release
	body    []byte
}

func (f *fakeRepo) GetLatestRelease(context.Context) (*repository.Release, error) {
	return f.release, nil
}

func (f *fakeRepo) CompareVersions(current, latest string) (bool, error) {
	return version.IsNewer(latest, current)
}

func (f *fakeRepo) Download(_ context.Context, _ *repository.Release, dest string) error {
	return os.WriteFile(dest, f.body, 0600)
}

// release describes payload the way a provider would: an https artifact URL and
// an "algorithm:hex" checksum, which is the only form parseDigest emits.
func release(version string, payload []byte) *repository.Release {
	name := fmt.Sprintf("app-%s.tar.gz", version)
	return &repository.Release{
		Version:     version,
		DownloadURL: "https://example.com/" + name,
		Checksum:    "sha256:" + sha256Hex(payload),
		FileName:    name,
	}
}

// newApp writes an app config and loads it back.
func newApp(t *testing.T, name string, mutate func(*config.App)) *config.App {
	t.Helper()

	a := config.New(name)
	a.Repository = config.RepositoryConfig{Type: "github", Owner: "example", Repo: name}
	a.Applier = "archive"
	if mutate != nil {
		mutate(a)
	}
	if err := a.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := config.LoadApp(name)
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	return loaded
}

// installer returns an Installer that hands out rel and serves payload for it.
func installer(out *bytes.Buffer, rel *repository.Release, payload []byte) *Installer {
	return &Installer{Repo: &fakeRepo{release: rel, body: payload}, Out: out}
}

// The full pipeline: download, verify, hooks, extract, link, record version.
func TestInstallArchiveEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks in this test use POSIX shell syntax")
	}
	testRoot(t)

	payload := tarGz(t, map[string]string{
		"hello-1.0.0/hello":  "#!/bin/sh\necho hello\n",
		"hello-1.0.0/README": "docs",
	})

	hookLog := filepath.Join(t.TempDir(), "hooks.log")
	a := newApp(t, "hello", func(a *config.App) {
		a.Bin = []string{"hello"}
		a.PreInstall = []string{`echo "pre $GUPPY_APP $GUPPY_VERSION" >> ` + hookLog}
		a.PostInstall = []string{`echo "post $GUPPY_VERSION $GUPPY_INSTALL_DIR" >> ` + hookLog}
	})

	var out bytes.Buffer
	if err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() error: %v\noutput:\n%s", err, out.String())
	}

	// The binary is linked and reachable through the bin directory.
	binDir, _ := config.BinDir()
	link := filepath.Join(binDir, "hello")
	info, err := os.Stat(link)
	if err != nil {
		t.Fatalf("bin link missing: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("linked binary mode = %v, want executable", info.Mode().Perm())
	}

	// It resolves through the versioned directory the archive unpacked into.
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("EvalSymlinks() error: %v", err)
	}
	if !strings.Contains(resolved, "hello-1.0.0") {
		t.Errorf("link resolves to %q, want the extracted versioned directory", resolved)
	}

	// Both hooks ran, in order, with the environment populated.
	logged, err := os.ReadFile(hookLog)
	if err != nil {
		t.Fatalf("hooks did not run: %v", err)
	}
	installDir, _ := config.InstallDir("hello")
	want := fmt.Sprintf("pre hello 1.0.0\npost 1.0.0 %s\n", installDir)
	if string(logged) != want {
		t.Errorf("hook log = %q, want %q", logged, want)
	}

	// The version was recorded, so a second run is a no-op.
	reloaded, err := config.LoadApp("hello")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if reloaded.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want 1.0.0", reloaded.CurrentVersion)
	}

	out.Reset()
	if err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), reloaded); err != nil {
		t.Fatalf("second Install() error: %v", err)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("second Install() output = %q, want an up-to-date message", out.String())
	}

	// Downloads are cleaned up rather than accumulating.
	downloadDir, _ := config.DownloadDir()
	entries, err := os.ReadDir(downloadDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("download directory still holds %d file(s)", len(entries))
	}
}

func TestInstallUpgradeReplacesOldVersion(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-2.0.0/app": "v2"})
	a := newApp(t, "app", func(a *config.App) {
		a.CurrentVersion = "1.0.0"
		a.Bin = []string{"app"}
	})

	// Stand in an old install, so the swap has something to replace.
	installDir, _ := config.InstallDir("app")
	if err := os.MkdirAll(filepath.Join(installDir, "app-1.0.0"), 0755); err != nil {
		t.Fatalf("failed to seed the old install: %v", err)
	}

	var out bytes.Buffer
	if err := installer(&out, release("2.0.0", payload), payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	binDir, _ := config.BinDir()
	resolved, err := filepath.EvalSymlinks(filepath.Join(binDir, "app"))
	if err != nil {
		t.Fatalf("EvalSymlinks() error: %v", err)
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("failed to read linked binary: %v", err)
	}
	if string(content) != "v2" {
		t.Errorf("linked binary = %q, want the 2.0.0 payload", content)
	}

	// The previous version's directory is gone, not left alongside the new one.
	if _, err := os.Stat(filepath.Join(installDir, "app-1.0.0")); !os.IsNotExist(err) {
		t.Error("old version directory survived the upgrade")
	}
}

func TestInstallPreInstallFailureAborts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks in this test use POSIX shell syntax")
	}
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
		a.PreInstall = []string{"exit 3"}
	})

	var out bytes.Buffer
	err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), a)
	if err == nil {
		t.Fatal("Install() succeeded, want the pre_install failure to abort it")
	}
	if !strings.Contains(err.Error(), "pre_install") {
		t.Errorf("Install() error = %v, want it to name pre_install", err)
	}

	// Nothing was installed and no version was recorded.
	binDir, _ := config.BinDir()
	if _, err := os.Lstat(filepath.Join(binDir, "app")); !os.IsNotExist(err) {
		t.Error("pre_install failed but a binary was still linked")
	}
	reloaded, err := config.LoadApp("app")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if reloaded.CurrentVersion != "" {
		t.Errorf("CurrentVersion = %q, want it left unset", reloaded.CurrentVersion)
	}
}

// A failing post_install is reported, but the install itself already happened
// and must be recorded so the next run doesn't redo it.
func TestInstallPostInstallFailureKeepsInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks in this test use POSIX shell syntax")
	}
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
		a.PostInstall = []string{"exit 7"}
	})

	var out bytes.Buffer
	err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), a)
	if err == nil {
		t.Fatal("Install() succeeded, want the post_install failure reported")
	}
	if !strings.Contains(err.Error(), "post_install") {
		t.Errorf("Install() error = %v, want it to name post_install", err)
	}

	binDir, _ := config.BinDir()
	if _, err := os.Stat(filepath.Join(binDir, "app")); err != nil {
		t.Errorf("post_install failed but the binary is not installed: %v", err)
	}
	reloaded, err := config.LoadApp("app")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if reloaded.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want 1.0.0 recorded despite the hook failure", reloaded.CurrentVersion)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	rel := release("1.0.0", payload)
	rel.Checksum = "sha256:" + strings.Repeat("0", 64)

	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
	})

	var out bytes.Buffer
	err := installer(&out, rel, payload).Install(context.Background(), a)
	if err == nil {
		t.Fatal("Install() succeeded on a bad checksum, want an error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("Install() error = %v, want a checksum failure", err)
	}

	installDir, _ := config.InstallDir("app")
	if entries, err := os.ReadDir(installDir); err == nil && len(entries) > 0 {
		t.Error("a corrupted download was still installed")
	}
}

// A release with no checksum is unverifiable, so guppy refuses it rather than
// installing something it cannot vouch for onto the user's PATH.
func TestInstallRefusesReleaseWithoutChecksum(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	rel := release("1.0.0", payload)
	rel.Checksum = ""

	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
	})

	var out bytes.Buffer
	err := installer(&out, rel, payload).Install(context.Background(), a)
	if err == nil {
		t.Fatal("Install() succeeded on a release with no checksum, want an error")
	}
	if !strings.Contains(err.Error(), "allow_unverified") {
		t.Errorf("Install() error = %v, want it to name the allow_unverified override", err)
	}

	installDir, _ := config.InstallDir("app")
	if entries, err := os.ReadDir(installDir); err == nil && len(entries) > 0 {
		t.Error("an unverifiable release was still installed")
	}
}

// The override exists, but it has to be loud: a warning only visible under
// --debug is not a warning.
func TestInstallAllowUnverifiedWarnsOnStdout(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	rel := release("1.0.0", payload)
	rel.Checksum = ""

	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
		a.AllowUnverified = true
	})

	var out bytes.Buffer
	if err := installer(&out, rel, payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() with allow_unverified error: %v", err)
	}
	if !strings.Contains(out.String(), "unverified") {
		t.Errorf("output = %q, want a visible unverified warning", out.String())
	}
	if a.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want the install to have completed", a.CurrentVersion)
	}
}

// A release names its own artifact URL, independently of the channel the
// release data arrived over, so an insecure artifact URL has to be caught too.
func TestInstallRejectsInsecureArtifactURL(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	rel := release("1.0.0", payload)
	rel.DownloadURL = "http://example.com/app.tar.gz"

	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
	})

	var out bytes.Buffer
	err := installer(&out, rel, payload).Install(context.Background(), a)
	if err == nil {
		t.Fatal("Install() accepted a plain-http artifact URL, want an error")
	}
	if !strings.Contains(err.Error(), "insecure") {
		t.Errorf("Install() error = %v, want it to flag the insecure URL", err)
	}
}

// Checksums arrive as "sha256:hex". Before consolidation those reached a
// bare-hex comparison and every such release failed verification.
func TestInstallAcceptsPrefixedChecksum(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
	})

	var out bytes.Buffer
	if err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !strings.Contains(out.String(), "checksum verified") {
		t.Errorf("output = %q, want the checksum to have been verified", out.String())
	}
}

func TestInstallBinaryApplier(t *testing.T) {
	testRoot(t)

	payload := []byte("#!/bin/sh\necho tool\n")
	rel := &repository.Release{
		Version:     "1.2.3",
		DownloadURL: "https://example.com/tool",
		Checksum:    "sha256:" + sha256Hex(payload),
		FileName:    "tool",
	}

	a := newApp(t, "tool", func(a *config.App) {
		a.Applier = "binary"
	})

	var out bytes.Buffer
	if err := installer(&out, rel, payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// The downloaded file lands in the install directory under the app name,
	// and the bin entry links to it.
	installDir, _ := config.InstallDir("tool")
	if _, err := os.Stat(filepath.Join(installDir, "tool")); err != nil {
		t.Errorf("binary not installed: %v", err)
	}

	binDir, _ := config.BinDir()
	content, err := os.ReadFile(filepath.Join(binDir, "tool"))
	if err != nil {
		t.Fatalf("failed to read linked binary: %v", err)
	}
	if !bytes.Equal(content, payload) {
		t.Errorf("linked binary = %q, want the downloaded payload", content)
	}
}

func TestCheck(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-2.0.0/app": "v2"})

	tests := []struct {
		name    string
		current string
		want    string
	}{
		{"not installed", "", "not installed"},
		{"update available", "1.0.0", "2.0.0 available"},
		{"up to date", "2.0.0", "up to date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := config.New("checkapp")
			a.Repository = config.RepositoryConfig{Type: "github", Owner: "example", Repo: "checkapp"}
			a.Applier = "archive"
			a.CurrentVersion = tt.current

			var out bytes.Buffer
			if err := installer(&out, release("2.0.0", payload), payload).Check(context.Background(), a); err != nil {
				t.Fatalf("Check() error: %v", err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("Check() output = %q, want it to mention %q", out.String(), tt.want)
			}

			// Check must not install anything.
			binDir, _ := config.BinDir()
			if _, err := os.Lstat(filepath.Join(binDir, "checkapp")); !os.IsNotExist(err) {
				t.Error("Check() installed a binary")
			}
		})
	}
}

func TestRemove(t *testing.T) {
	testRoot(t)

	payload := tarGz(t, map[string]string{"app-1.0.0/app": "v1"})
	a := newApp(t, "app", func(a *config.App) {
		a.Bin = []string{"app"}
	})

	var out bytes.Buffer
	if err := installer(&out, release("1.0.0", payload), payload).Install(context.Background(), a); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if err := Remove(a); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	binDir, _ := config.BinDir()
	if _, err := os.Lstat(filepath.Join(binDir, "app")); !os.IsNotExist(err) {
		t.Error("Remove() left the bin link behind")
	}

	installDir, _ := config.InstallDir("app")
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Error("Remove() left the install directory behind")
	}

	if _, err := config.AppPath("app"); err == nil {
		t.Error("Remove() left the config file behind")
	}
}

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name    string
		app     config.App
		wantErr bool
	}{
		{name: "github", app: config.App{Repository: config.RepositoryConfig{Type: "github", Owner: "o", Repo: "r"}}},
		{name: "github with asset", app: config.App{Repository: config.RepositoryConfig{Type: "github", Owner: "o", Repo: "r", AssetName: "x.tar.gz"}}},
		{name: "http is no longer a provider", app: config.App{Repository: config.RepositoryConfig{Type: "http"}}, wantErr: true},
		{name: "unknown", app: config.App{Repository: config.RepositoryConfig{Type: "ftp"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewRepository(&tt.app, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRepository() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && repo == nil {
				t.Error("NewRepository() returned a nil repository")
			}
		})
	}
}
