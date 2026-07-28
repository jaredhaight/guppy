package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jaredhaight/guppy/internal/app"
	"github.com/jaredhaight/guppy/internal/config"
	"github.com/jaredhaight/guppy/internal/util"
	"github.com/spf13/cobra"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	// One signal handler for the whole program. Cancelling the context is
	// what makes Ctrl-C interrupt an in-flight download rather than waiting
	// out the HTTP client timeout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// stop() restores the default disposition, so a second Ctrl-C kills the
	// process outright. Without it an unresponsive transfer would be
	// uninterruptible after the first signal.
	go func() {
		<-ctx.Done()
		stop()
	}()
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// cli carries the flags every command shares.
//
// These were package-level variables. Holding them on a value built per
// invocation is what lets the tests construct a command, point it at a
// buffer, and run it without resetting global state between cases.
type cli struct {
	configDir string
	debug     bool
	interval  string
}

func (c *cli) debugf(cmd *cobra.Command, format string, args ...any) {
	if c.debug {
		fmt.Fprintf(cmd.ErrOrStderr(), "[DEBUG] "+format+"\n", args...)
	}
}

// newInstaller builds an installer that writes to the command's output, so a
// test can capture it without touching os.Stdout.
func (c *cli) newInstaller(cmd *cobra.Command, a *config.App) (*app.Installer, error) {
	repo, err := app.NewRepository(a, c.debug)
	if err != nil {
		return nil, err
	}
	return &app.Installer{Repo: repo, Out: cmd.OutOrStdout(), Debug: c.debug}, nil
}

func newRootCmd() *cobra.Command {
	c := &cli{}

	root := &cobra.Command{
		Use:   "guppy [app...]",
		Short: "Guppy is a software update helper",
		Long: `Guppy checks for new releases of the applications it manages, downloads
them, and installs their binaries into a folder on your PATH.

With no arguments it updates every configured app.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if c.configDir != "" {
				return os.Setenv(config.EnvConfigDir, c.configDir)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no interval flag is set, run once
			if c.interval == "" {
				return c.updateApps(cmd, args)
			}
			return c.watch(cmd, args)
		},
	}

	root.PersistentFlags().StringVar(&c.configDir, "config-dir", "", "directory holding guppy's app configs")
	root.PersistentFlags().BoolVarP(&c.debug, "debug", "d", false, "enable debug logging")
	root.Flags().StringVarP(&c.interval, "interval", "i", "", "check for updates at regular intervals (e.g., 15m, 1h, 1d, or HH:MM:SS)")

	root.AddCommand(
		c.newCheckCmd(),
		newVersionCmd(),
		c.newAddCmd(),
		c.newListCmd(),
		c.newRemoveCmd(),
		c.newBinCmd(),
	)
	return root
}

// watch runs update checks on a timer until the context is cancelled.
func (c *cli) watch(cmd *cobra.Command, args []string) error {
	interval, err := util.ParseInterval(c.interval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	// Already cancelled on SIGINT/SIGTERM by main, so waiting on it both ends
	// the loop and aborts whatever download is in flight.
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Fprintf(out, "Starting update monitoring (checking every %s). Press Ctrl+C to stop.\n", interval)

	runOnce := func() {
		if err := c.updateApps(cmd, args); err != nil {
			if ctx.Err() != nil {
				return // shutting down; the error is the cancellation
			}
			fmt.Fprintf(out, "Error during update check: %v\n", err)
			fmt.Fprintln(out, "Will retry at next interval...")
		}
	}

	runOnce()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out, "\nReceived shutdown signal, stopping...")
			return nil
		case <-ticker.C:
			fmt.Fprintf(out, "\n[%s] Running scheduled update check...\n", time.Now().Format("2006-01-02 15:04:05"))
			runOnce()
		}
	}
}

func (c *cli) newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [app...]",
		Short: "Check for available updates without installing them",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			apps, err := resolveApps(args)
			if err != nil {
				return err
			}

			return c.forEachApp(cmd, apps, func(a *config.App) error {
				installer, err := c.newInstaller(cmd, a)
				if err != nil {
					return err
				}
				return installer.Check(cmd.Context(), a)
			})
		},
	}
}

func (c *cli) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List managed applications",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			names, err := config.ListApps()
			if err != nil {
				return err
			}

			if len(names) == 0 {
				dir, _ := config.AppsDir()
				fmt.Fprintf(out, "No apps configured. Add one with:\n\n  guppy add <owner>/<repo>\n\nConfigs live in %s\n", dir)
				return nil
			}

			fmt.Fprintf(out, "%-24s %-14s %s\n", "APP", "VERSION", "SOURCE")
			for _, name := range names {
				a, err := config.LoadApp(name)
				if err != nil {
					fmt.Fprintf(out, "%-24s %-14s %v\n", name, "?", err)
					continue
				}

				version := a.CurrentVersion
				if version == "" {
					version = "-"
				}

				source := a.Repository.Owner + "/" + a.Repository.Repo

				fmt.Fprintf(out, "%-24s %-14s %s\n", name, version, source)
			}
			return nil
		},
	}
}

func (c *cli) newBinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bin",
		Short: "Print the directory guppy links binaries into",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.BinDir()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), dir)
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show guppy version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Guppy Software Updater\n")
			fmt.Fprint(out, "https://www.github.com/jaredhaight/guppy\n")
			fmt.Fprintf(out, "Version: %s\n", Version)
		},
	}
}

// addFlags are the flags that describe a new app. Both add and install take
// them, so they're registered from one place rather than declared twice.
type addFlags struct {
	name        string
	applier     string
	bin         []string
	asset       string
	token       string
	preInstall  []string
	postInstall []string
}

// register declares the app-describing flags on cmd.
func (f *addFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "name for the app (defaults to the repo name)")
	cmd.Flags().StringVar(&f.applier, "applier", "binary", "how to install the download (binary or archive)")
	cmd.Flags().StringArrayVar(&f.bin, "bin", nil, "binary to link into the bin directory (repeatable)")
	cmd.Flags().StringVar(&f.asset, "asset", "", "release asset name to download")
	cmd.Flags().StringVar(&f.token, "token", "", "GitHub token for private repos or higher rate limits")
	cmd.Flags().StringArrayVar(&f.preInstall, "pre-install", nil, "shell command to run before installing (repeatable)")
	cmd.Flags().StringArrayVar(&f.postInstall, "post-install", nil, "shell command to run after installing (repeatable)")

	// Only errors if the flag is missing, which would be a bug on the line
	// above rather than anything that can happen at runtime.
	_ = cmd.RegisterFlagCompletionFunc("applier",
		cobra.FixedCompletions([]string{"binary", "archive"}, cobra.ShellCompDirectiveNoFileComp))
}

// changed reports whether any app-describing flag was given. It asks cobra
// rather than inspecting the values because --applier has a default, so it
// always looks set.
func (f *addFlags) changed(cmd *cobra.Command) bool {
	for _, name := range []string{"name", "applier", "bin", "asset", "token", "pre-install", "post-install"} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func (c *cli) newAddCmd() *cobra.Command {
	f := &addFlags{}

	cmd := &cobra.Command{
		Use:   "add <owner>/<repo>",
		Short: "Add an application for guppy to manage, without installing it",
		Long: `Add an application for guppy to manage. This writes the config and
downloads nothing; run 'guppy install <app>' to install it.

  guppy add BurntSushi/ripgrep --applier archive --bin rg`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				return fmt.Errorf("expected owner/repo\n\nUsage: guppy add <owner>/<repo>")
			}

			a, name, err := f.buildApp(cmd, args[0])
			if err != nil {
				return err
			}

			if _, err := config.AppPath(name); err == nil {
				return fmt.Errorf("app %q already exists", name)
			}

			if err := a.Validate(); err != nil {
				return err
			}

			if err := a.Save(); err != nil {
				return err
			}

			fmt.Fprintf(out, "✓ Added %s (%s)\n", name, a.Path())

			binDir, err := config.BinDir()
			if err == nil {
				fmt.Fprintf(out, "\nRun 'guppy %s' to install it. Binaries are linked into:\n  %s\n", name, binDir)
				if !onPath(binDir) {
					fmt.Fprintf(out, "\nThat directory isn't on your PATH yet. Add it with:\n  export PATH=\"%s:$PATH\"\n", binDir)
				}
			}
			return nil
		},
	}

	f.register(cmd)

	return cmd
}

// buildApp turns an owner/repo spec plus the add flags into an app config.
func (f *addFlags) buildApp(cmd *cobra.Command, spec string) (*config.App, string, error) {
	owner, repoName, found := strings.Cut(spec, "/")
	if !found || owner == "" || repoName == "" {
		return nil, "", fmt.Errorf("expected owner/repo, got %q", spec)
	}

	name := f.name
	if name == "" {
		name = repoName
	}
	repo := config.RepositoryConfig{
		Type:      "github",
		Owner:     owner,
		Repo:      repoName,
		Token:     f.token,
		AssetName: f.asset,
	}

	if err := config.ValidateName(name); err != nil {
		return nil, "", err
	}
	if reserved(cmd, name) {
		return nil, "", fmt.Errorf("%q is a guppy command; use --name to pick a different app name", name)
	}

	a := config.New(name)
	a.Repository = repo
	a.Applier = f.applier
	a.Bin = f.bin
	a.PreInstall = f.preInstall
	a.PostInstall = f.postInstall

	return a, name, nil
}

func (c *cli) newRemoveCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "remove <app>",
		Aliases: []string{"rm"},
		Short:   "Remove an application and everything guppy installed for it",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			a, err := config.LoadApp(args[0])
			if err != nil {
				return err
			}

			installDir, err := config.InstallDir(a.Name())
			if err != nil {
				return err
			}

			if !yes {
				fmt.Fprintf(out, "This will delete:\n  %s\n  %s\nand unlink %s from the bin directory.\n",
					a.Path(), installDir, strings.Join(a.Binaries(), ", "))
				fmt.Fprint(out, "Continue? [y/N]: ")

				var answer string
				_, _ = fmt.Fscanln(cmd.InOrStdin(), &answer)
				if !strings.EqualFold(strings.TrimSpace(answer), "y") {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
			}

			if err := app.Remove(a); err != nil {
				return err
			}

			fmt.Fprintf(out, "✓ Removed %s\n", a.Name())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation")
	return cmd
}

// reserved reports whether a name would be shadowed by a guppy subcommand.
func reserved(cmd *cobra.Command, name string) bool {
	for _, sub := range cmd.Root().Commands() {
		if sub.Name() == name || slices.Contains(sub.Aliases, name) {
			return true
		}
	}
	return false
}

// resolveApps returns the named apps, or every configured app when no names
// are given.
func resolveApps(names []string) ([]string, error) {
	if len(names) > 0 {
		return names, nil
	}

	all, err := config.ListApps()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no apps configured\n\nAdd one with: guppy add <owner>/<repo>")
	}
	return all, nil
}

// forEachApp runs fn against each named app, continuing past failures so one
// broken app doesn't stop the rest, and reporting how many failed.
func (c *cli) forEachApp(cmd *cobra.Command, names []string, fn func(*config.App) error) error {
	errOut := cmd.ErrOrStderr()

	var failed int
	for _, name := range names {
		a, err := config.LoadApp(name)
		if err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", name, err)
			failed++
			continue
		}

		c.debugf(cmd, "Loaded app %s from %s", a.Name(), a.Path())
		if err := fn(a); err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", name, err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d app(s) failed", failed, len(names))
	}
	return nil
}

func (c *cli) updateApps(cmd *cobra.Command, args []string) error {
	apps, err := resolveApps(args)
	if err != nil {
		return err
	}

	return c.forEachApp(cmd, apps, func(a *config.App) error {
		installer, err := c.newInstaller(cmd, a)
		if err != nil {
			return err
		}
		return installer.Install(cmd.Context(), a)
	})
}

// onPath reports whether dir is already in PATH.
func onPath(dir string) bool {
	return slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir)
}
