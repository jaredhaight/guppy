package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaredhaight/guppy/internal/config"
	"github.com/spf13/cobra"
)

// testRoot points config and data at a temp dir so tests never touch the real
// home directory.
func testRoot(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv(config.EnvConfigDir, filepath.Join(root, "config"))
	t.Setenv(config.EnvDataDir, filepath.Join(root, "data"))
}

// run executes guppy with the given arguments, returning everything written to
// stdout and stderr.
//
// Each call builds a fresh command tree, so there is no flag state to reset
// between cases and no need to redirect the process's own stdout.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(args)

	// Execute first: return operands are evaluated left to right, so reading
	// the buffer in the return statement would read it before the command ran.
	err := cmd.Execute()
	return buf.String(), err
}

// testCLI returns a cli and a command whose output goes nowhere visible, for
// the helpers that are exercised directly rather than through run().
func testCLI() (*cli, *cobra.Command) {
	cmd := newRootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return &cli{}, cmd
}

func TestAddGitHub(t *testing.T) {
	testRoot(t)

	out, err := run(t, "add", "BurntSushi/ripgrep", "--applier", "archive", "--bin", "rg")
	if err != nil {
		t.Fatalf("add error: %v (%s)", err, out)
	}

	// The app name defaults to the repo name.
	a, err := config.LoadApp("ripgrep")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if a.Repository.Type != "github" || a.Repository.Owner != "BurntSushi" || a.Repository.Repo != "ripgrep" {
		t.Errorf("repository = %+v", a.Repository)
	}
	if a.Applier != "archive" {
		t.Errorf("Applier = %q, want archive", a.Applier)
	}
	if len(a.Bin) != 1 || a.Bin[0] != "rg" {
		t.Errorf("Bin = %v, want [rg]", a.Bin)
	}
	if filepath.Ext(a.Path()) != ".yaml" {
		t.Errorf("add wrote %q, want a .yaml file", a.Path())
	}
}

func TestAddWithHooks(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/repo",
		"--pre-install", "systemctl stop x",
		"--post-install", "echo one",
		"--post-install", "echo two"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	a, err := config.LoadApp("repo")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if len(a.PreInstall) != 1 || a.PreInstall[0] != "systemctl stop x" {
		t.Errorf("PreInstall = %v", a.PreInstall)
	}
	if len(a.PostInstall) != 2 || a.PostInstall[1] != "echo two" {
		t.Errorf("PostInstall = %v, want both commands in order", a.PostInstall)
	}
}

func TestAddHTTP(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "--url", "https://example.com/releases.json", "--name", "myapp"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	a, err := config.LoadApp("myapp")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if a.Repository.Type != "http" || a.Repository.URL != "https://example.com/releases.json" {
		t.Errorf("repository = %+v", a.Repository)
	}
}

func TestAddErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "http without a name",
			args: []string{"add", "--url", "https://example.com/r.json"},
			want: "--name is required",
		},
		{
			name: "no arguments",
			args: []string{"add"},
			want: "expected owner/repo",
		},
		{
			name: "malformed repo spec",
			args: []string{"add", "justarepo"},
			want: "expected owner/repo",
		},
		{
			name: "empty owner",
			args: []string{"add", "/repo"},
			want: "expected owner/repo",
		},
		{
			name: "both repo and url",
			args: []string{"add", "owner/repo", "--url", "https://example.com/r.json"},
			want: "not both",
		},
		{
			name: "name shadowing a subcommand",
			args: []string{"add", "owner/list"},
			want: "is a guppy command",
		},
		{
			name: "name escaping the apps directory",
			args: []string{"add", "--url", "https://example.com/r.json", "--name", "../evil"},
			want: "invalid app name",
		},
		{
			name: "unknown applier",
			args: []string{"add", "owner/repo", "--applier", "magic"},
			want: "invalid applier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRoot(t)

			_, err := run(t, tt.args...)
			if err == nil {
				t.Fatalf("%v succeeded, want an error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestAddDuplicate(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/repo"); err != nil {
		t.Fatalf("first add error: %v", err)
	}
	_, err := run(t, "add", "owner/repo")
	if err == nil {
		t.Fatal("second add succeeded, want an already-exists error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want an already-exists error", err)
	}
}

func TestListEmpty(t *testing.T) {
	testRoot(t)

	out, err := run(t, "list")
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if !strings.Contains(out, "No apps configured") {
		t.Errorf("list output = %q, want a hint for empty configuration", out)
	}
}

func TestList(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "BurntSushi/ripgrep"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := run(t, "add", "--url", "https://example.com/r.json", "--name", "myapp"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	out, err := run(t, "list")
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	for _, want := range []string{"ripgrep", "BurntSushi/ripgrep", "myapp", "https://example.com/r.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
}

func TestBinCmd(t *testing.T) {
	testRoot(t)

	out, err := run(t, "bin")
	if err != nil {
		t.Fatalf("bin error: %v", err)
	}

	want, err := config.BinDir()
	if err != nil {
		t.Fatalf("BinDir() error: %v", err)
	}
	if strings.TrimSpace(out) != want {
		t.Errorf("bin output = %q, want %q", strings.TrimSpace(out), want)
	}
}

func TestRemoveCmd(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/repo"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := run(t, "remove", "repo", "--yes"); err != nil {
		t.Fatalf("remove error: %v", err)
	}
	if _, err := config.AppPath("repo"); err == nil {
		t.Error("remove left the config file behind")
	}
}

// The confirmation prompt reads from the command's input rather than the
// process's stdin, which is both why it is testable and how a "no" answer is
// honored.
func TestRemoveConfirmationPrompt(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		removed bool
	}{
		{"y confirms", "y\n", true},
		{"uppercase Y confirms", "Y\n", true},
		{"n declines", "n\n", false},
		{"empty declines", "\n", false},
		{"anything else declines", "yes please\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRoot(t)
			if _, err := run(t, "add", "owner/repo"); err != nil {
				t.Fatalf("add error: %v", err)
			}

			var buf bytes.Buffer
			cmd := newRootCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetIn(strings.NewReader(tt.answer))
			cmd.SetArgs([]string{"remove", "repo"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("remove error: %v", err)
			}

			_, err := config.AppPath("repo")
			gone := err != nil
			if gone != tt.removed {
				t.Errorf("answer %q: removed = %v, want %v (output: %s)", tt.answer, gone, tt.removed, buf.String())
			}
			if !tt.removed && !strings.Contains(buf.String(), "Cancelled") {
				t.Errorf("answer %q: output = %q, want a cancellation notice", tt.answer, buf.String())
			}
		})
	}
}

func TestRemoveMissingApp(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "remove", "ghost", "--yes"); err == nil {
		t.Error("remove of a missing app succeeded, want an error")
	}
}

func TestConfigDirFlag(t *testing.T) {
	testRoot(t)
	custom := t.TempDir()

	if _, err := run(t, "--config-dir", custom, "add", "owner/repo"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(custom, "apps", "repo.yaml")); err != nil {
		t.Errorf("--config-dir was not honored: %v", err)
	}
}

func TestResolveAppsDefaultsToAll(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/alpha"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := run(t, "add", "owner/beta"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	names, err := resolveApps(nil)
	if err != nil {
		t.Fatalf("resolveApps() error: %v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("resolveApps(nil) = %v, want all apps sorted", names)
	}

	// Explicit names are passed through untouched.
	got, err := resolveApps([]string{"beta"})
	if err != nil {
		t.Fatalf("resolveApps() error: %v", err)
	}
	if len(got) != 1 || got[0] != "beta" {
		t.Errorf("resolveApps([beta]) = %v", got)
	}
}

func TestResolveAppsWithNoneConfigured(t *testing.T) {
	testRoot(t)

	if _, err := resolveApps(nil); err == nil {
		t.Error("resolveApps() with no apps succeeded, want a helpful error")
	}
}

// One broken app must not stop the others from being processed.
func TestForEachAppContinuesPastFailures(t *testing.T) {
	testRoot(t)

	for _, spec := range []string{"owner/alpha", "owner/beta", "owner/gamma"} {
		if _, err := run(t, "add", spec); err != nil {
			t.Fatalf("add %s error: %v", spec, err)
		}
	}

	var seen []string
	c, cmd := testCLI()
	err := c.forEachApp(cmd, []string{"alpha", "beta", "gamma"}, func(a *config.App) error {
		seen = append(seen, a.Name())
		if a.Name() == "beta" {
			return os.ErrInvalid
		}
		return nil
	})

	if err == nil {
		t.Error("forEachApp() returned nil, want it to report the failure")
	}
	if len(seen) != 3 {
		t.Errorf("visited %v, want all three apps despite the failure", seen)
	}
	if !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("error = %v, want a count of failures", err)
	}
}

func TestForEachAppReportsUnloadableApps(t *testing.T) {
	testRoot(t)

	appsDir, err := config.AppsDir()
	if err != nil {
		t.Fatalf("AppsDir() error: %v", err)
	}
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatalf("failed to create apps dir: %v", err)
	}
	// Missing the required owner/repo for a github app.
	if err := os.WriteFile(filepath.Join(appsDir, "broken.yaml"), []byte("repository:\n  type: github\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	called := false
	c, cmd := testCLI()
	err = c.forEachApp(cmd, []string{"broken"}, func(a *config.App) error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("forEachApp() with an invalid config returned nil, want an error")
	}
	if called {
		t.Error("forEachApp() invoked the callback for an app that failed to load")
	}
}

func TestReserved(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"list", true},
		{"add", true},
		{"remove", true},
		{"rm", true}, // alias
		{"check", true},
		{"bin", true},
		{"version", true},
		{"ripgrep", false},
		{"myapp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reserved(newRootCmd(), tt.name); got != tt.want {
				t.Errorf("reserved(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestOnPath(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()

	t.Setenv("PATH", strings.Join([]string{"/usr/bin", dir, "/bin"}, string(os.PathListSeparator)))

	if !onPath(dir) {
		t.Errorf("onPath(%q) = false, want true", dir)
	}
	if onPath(other) {
		t.Errorf("onPath(%q) = true, want false", other)
	}
}

func TestDebugf(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetErr(&buf)

	c := &cli{debug: true}
	c.debugf(cmd, "test message: %s", "hello")

	c.debug = false
	c.debugf(cmd, "should not appear")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG] test message: hello") {
		t.Errorf("debugf() output = %q, want the message when debug is on", output)
	}
	if strings.Contains(output, "should not appear") {
		t.Errorf("debugf() output = %q, want nothing when debug is off", output)
	}
}

func TestVersionCmd(t *testing.T) {
	oldVersion := Version
	Version = "v1.2.3-test"
	defer func() { Version = oldVersion }()

	out, err := run(t, "version")
	if err != nil {
		t.Fatalf("version error: %v", err)
	}
	if !strings.Contains(out, "v1.2.3-test") {
		t.Errorf("version output = %q, want the version string", out)
	}
}
