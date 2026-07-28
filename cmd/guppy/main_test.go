package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
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

	// Never nil: cobra reads os.Args when SetArgs is given nil, which would
	// hand the test binary's own flags to guppy.
	if args == nil {
		args = []string{}
	}
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

func TestAddRenamesWithName(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "BurntSushi/ripgrep", "--name", "myrg"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	a, err := config.LoadApp("myrg")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if a.Repository.Owner != "BurntSushi" || a.Repository.Repo != "ripgrep" {
		t.Errorf("repository = %+v, want the repo it was renamed from", a.Repository)
	}
}

func TestAddErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
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
			name: "name shadowing a subcommand",
			args: []string{"add", "owner/list"},
			want: "is a guppy command",
		},
		{
			name: "name escaping the apps directory",
			args: []string{"add", "owner/repo", "--name", "../evil"},
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
	if _, err := run(t, "add", "sharkdp/fd", "--name", "myapp"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	out, err := run(t, "list")
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	for _, want := range []string{"ripgrep", "BurntSushi/ripgrep", "myapp", "sharkdp/fd"} {
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
		{"install", true},
		{"update", true},
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

	// Cobra adds `completion` and `help` while executing, not while the tree
	// is built, so those two are only reserved along the real path.
	t.Run("commands cobra adds during execute", func(t *testing.T) {
		for _, name := range []string{"completion", "help"} {
			testRoot(t)
			_, err := run(t, "add", "owner/"+name)
			if err == nil {
				t.Errorf("add owner/%s succeeded, want it rejected as a guppy command", name)
				continue
			}
			if !strings.Contains(err.Error(), "is a guppy command") {
				t.Errorf("add owner/%s error = %v, want a shadowing error", name, err)
			}
		}
	})
}

// A bare guppy used to update every app. It explains itself instead.
func TestBareGuppyPrintsHelp(t *testing.T) {
	testRoot(t)

	out, err := run(t)
	if err != nil {
		t.Fatalf("bare guppy error: %v", err)
	}
	for _, want := range []string{"Available Commands", "install", "update"} {
		if !strings.Contains(out, want) {
			t.Errorf("bare guppy output missing %q:\n%s", want, out)
		}
	}
}

// The old `guppy <app>` form has to fail rather than silently do nothing.
func TestUnknownCommandIsAnError(t *testing.T) {
	testRoot(t)

	_, err := run(t, "ripgrep")
	if err == nil {
		t.Fatal("guppy ripgrep succeeded, want an unknown-command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %v, want an unknown-command error", err)
	}
}

// --interval belongs to update now, not to the root.
func TestRootRejectsIntervalFlag(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "--interval", "1h"); err == nil {
		t.Fatal("guppy --interval succeeded, want the flag to have moved to update")
	}

	// And update still accepts it. A bad value proves it parsed the flag
	// rather than reaching the network.
	_, err := run(t, "update", "--interval", "not-a-duration")
	if err == nil {
		t.Fatal("update --interval accepted a bad duration")
	}
	if !strings.Contains(err.Error(), "interval") {
		t.Errorf("error = %v, want it to name the interval", err)
	}
}

// installTarget is the part of install that decides what to install, and
// whether it has to be added first. Exercised directly so the tests stay off
// the network.
func newInstallTarget() (*cli, *addFlags, *cobra.Command) {
	f := &addFlags{}
	cmd := &cobra.Command{Use: "install"}
	cmd.SetOut(io.Discard)
	f.register(cmd)
	return &cli{}, f, cmd
}

func TestInstallTargetAddsRepo(t *testing.T) {
	testRoot(t)

	c, f, cmd := newInstallTarget()
	if err := cmd.Flags().Set("bin", "rg"); err != nil {
		t.Fatalf("failed to set --bin: %v", err)
	}

	name, err := c.installTarget(cmd, f, "BurntSushi/ripgrep")
	if err != nil {
		t.Fatalf("installTarget() error: %v", err)
	}
	if name != "ripgrep" {
		t.Errorf("installTarget() = %q, want the repo name", name)
	}

	a, err := config.LoadApp("ripgrep")
	if err != nil {
		t.Fatalf("installTarget() did not write a config: %v", err)
	}
	if a.Repository.Owner != "BurntSushi" || len(a.Bin) != 1 || a.Bin[0] != "rg" {
		t.Errorf("config = %+v, want the flags applied", a)
	}
}

func TestInstallTargetPassesThroughAppName(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/repo"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	c, f, cmd := newInstallTarget()
	name, err := c.installTarget(cmd, f, "repo")
	if err != nil {
		t.Fatalf("installTarget() error: %v", err)
	}
	if name != "repo" {
		t.Errorf("installTarget() = %q, want the name unchanged", name)
	}
}

func TestInstallTargetErrors(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		setFlag bool
		want    string
	}{
		{
			name: "repo guppy already manages",
			arg:  "owner/repo",
			want: "guppy update repo",
		},
		{
			name:    "app flags given with an app name",
			arg:     "repo",
			setFlag: true,
			want:    "already has a config",
		},
		{
			name: "malformed repo spec",
			arg:  "owner/",
			want: "expected owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRoot(t)
			if _, err := run(t, "add", "owner/repo"); err != nil {
				t.Fatalf("add error: %v", err)
			}

			c, f, cmd := newInstallTarget()
			if tt.setFlag {
				if err := cmd.Flags().Set("applier", "archive"); err != nil {
					t.Fatalf("failed to set --applier: %v", err)
				}
			}

			_, err := c.installTarget(cmd, f, tt.arg)
			if err == nil {
				t.Fatalf("installTarget(%q) succeeded, want an error", tt.arg)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

// The split between install and update invites this, so it gets a real answer.
func TestUpdateRejectsOwnerRepo(t *testing.T) {
	testRoot(t)

	_, err := run(t, "update", "BurntSushi/ripgrep")
	if err == nil {
		t.Fatal("update accepted an owner/repo argument")
	}
	if !strings.Contains(err.Error(), "guppy install") {
		t.Errorf("error = %v, want it to point at install", err)
	}
}

func TestCompleteAppNames(t *testing.T) {
	testRoot(t)

	for _, spec := range []string{"owner/alpha", "owner/beta", "owner/gamma"} {
		if _, err := run(t, "add", spec); err != nil {
			t.Fatalf("add %s error: %v", spec, err)
		}
	}

	tests := []struct {
		name       string
		args       []string
		toComplete string
		want       []string
	}{
		{"all apps", nil, "", []string{"alpha", "beta", "gamma"}},
		{"filtered by prefix", nil, "b", []string{"beta"}},
		{"excludes what is already named", []string{"beta"}, "", []string{"alpha", "gamma"}},
		{"no match", nil, "z", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, cmd := testCLI()
			got, directive := c.completeAppNames(cmd, tt.args, tt.toComplete)
			if !slices.Equal(got, tt.want) {
				t.Errorf("completeAppNames() = %v, want %v", got, tt.want)
			}
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive = %v, want NoFileComp", directive)
			}
		})
	}
}

// Completion runs in a subprocess that is handed the whole command line, so
// --config-dir has to be honored there too. Cobra's __complete disables flag
// parsing, which means PersistentPreRunE fires before the flag has a value —
// the completion function has to publish it itself.
func TestCompletionHonorsConfigDir(t *testing.T) {
	testRoot(t)

	if _, err := run(t, "add", "owner/default"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	custom := t.TempDir()
	if _, err := run(t, "--config-dir", custom, "add", "owner/custom"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	out, err := run(t, "__complete", "--config-dir", custom, "update", "")
	if err != nil {
		t.Fatalf("__complete error: %v", err)
	}
	if !strings.Contains(out, "custom") {
		t.Errorf("__complete output = %q, want the app from --config-dir", out)
	}
	if strings.Contains(out, "default") {
		t.Errorf("__complete output = %q, want --config-dir to have been honored", out)
	}
}

func TestCompletionCommandExists(t *testing.T) {
	testRoot(t)

	out, err := run(t, "completion", "bash")
	if err != nil {
		t.Fatalf("completion error: %v", err)
	}
	if !strings.Contains(out, "guppy") {
		t.Errorf("completion output = %q, want a shell script", out)
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
