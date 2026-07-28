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

var (
	Version      = "dev"
	configDir    string
	debug        bool
	intervalFlag string
)

func main() {
	// One signal handler for the whole program. Cancelling the context is
	// what makes Ctrl-C interrupt an in-flight download rather than waiting
	// out the HTTP timeout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// stop() restores the default disposition, so a second Ctrl-C kills the
	// process outright. Without it an unresponsive transfer would be
	// uninterruptible after the first signal.
	go func() {
		<-ctx.Done()
		stop()
	}()
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// debugLog prints a debug message if debug mode is enabled
func debugLog(format string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

var rootCmd = &cobra.Command{
	Use:   "guppy [app...]",
	Short: "Guppy is a software update helper",
	Long: `Guppy checks for new releases of the applications it manages, downloads
them, and installs their binaries into a folder on your PATH.

With no arguments it updates every configured app.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no interval flag is set, run once
		if intervalFlag == "" {
			return updateApps(cmd.Context(), args)
		}

		interval, err := util.ParseInterval(intervalFlag)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}

		// Already cancelled on SIGINT/SIGTERM by main, so waiting on it both
		// ends the loop and aborts whatever download is in flight.
		ctx := cmd.Context()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		fmt.Printf("Starting update monitoring (checking every %s). Press Ctrl+C to stop.\n", interval)

		runOnce := func() {
			if err := updateApps(ctx, args); err != nil {
				if ctx.Err() != nil {
					return // shutting down; the error is the cancellation
				}
				fmt.Printf("Error during update check: %v\n", err)
				fmt.Println("Will retry at next interval...")
			}
		}

		runOnce()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("\nReceived shutdown signal, stopping...")
				return nil
			case <-ticker.C:
				fmt.Printf("\n[%s] Running scheduled update check...\n", time.Now().Format("2006-01-02 15:04:05"))
				runOnce()
			}
		}
	},
}

var checkCmd = &cobra.Command{
	Use:   "check [app...]",
	Short: "Check for available updates without installing them",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := resolveApps(args)
		if err != nil {
			return err
		}

		return forEachApp(apps, func(a *config.App) error {
			installer, err := newInstaller(a)
			if err != nil {
				return err
			}
			return installer.Check(cmd.Context(), a)
		})
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed applications",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := config.ListApps()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			dir, _ := config.AppsDir()
			fmt.Printf("No apps configured. Add one with:\n\n  guppy add <owner>/<repo>\n\nConfigs live in %s\n", dir)
			return nil
		}

		fmt.Printf("%-24s %-14s %s\n", "APP", "VERSION", "SOURCE")
		for _, name := range names {
			a, err := config.LoadApp(name)
			if err != nil {
				fmt.Printf("%-24s %-14s %v\n", name, "?", err)
				continue
			}

			version := a.CurrentVersion
			if version == "" {
				version = "-"
			}

			source := a.Repository.URL
			if a.Repository.Type == "github" {
				source = a.Repository.Owner + "/" + a.Repository.Repo
			}

			fmt.Printf("%-24s %-14s %s\n", name, version, source)
		}
		return nil
	},
}

var binCmd = &cobra.Command{
	Use:   "bin",
	Short: "Print the directory guppy links binaries into",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.BinDir()
		if err != nil {
			return err
		}
		fmt.Println(dir)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show guppy version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Guppy Software Updater\n")
		fmt.Print("https://www.github.com/jaredhaight/guppy\n")
		fmt.Printf("Version: %s\n", Version)
	},
}

var (
	addName        string
	addApplier     string
	addBin         []string
	addAsset       string
	addToken       string
	addURL         string
	addPreInstall  []string
	addPostInstall []string
)

var addCmd = &cobra.Command{
	Use:   "add [owner/repo]",
	Short: "Add an application for guppy to manage",
	Long: `Add an application for guppy to manage.

GitHub releases:
  guppy add BurntSushi/ripgrep --applier archive --bin rg

An HTTP releases.json feed:
  guppy add --url https://example.com/releases.json --name myapp`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, name, err := buildApp(args)
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

		fmt.Printf("✓ Added %s (%s)\n", name, a.Path())

		binDir, err := config.BinDir()
		if err == nil {
			fmt.Printf("\nRun 'guppy %s' to install it. Binaries are linked into:\n  %s\n", name, binDir)
			if !onPath(binDir) {
				fmt.Printf("\nThat directory isn't on your PATH yet. Add it with:\n  export PATH=\"%s:$PATH\"\n", binDir)
			}
		}
		return nil
	},
}

// buildApp turns the add flags into an app config.
func buildApp(args []string) (*config.App, string, error) {
	var (
		name string
		repo config.RepositoryConfig
	)

	switch {
	case addURL != "":
		if len(args) > 0 {
			return nil, "", fmt.Errorf("give either owner/repo or --url, not both")
		}
		if addName == "" {
			return nil, "", fmt.Errorf("--name is required with --url")
		}
		name = addName
		repo = config.RepositoryConfig{Type: "http", URL: addURL}

	case len(args) == 1:
		owner, repoName, found := strings.Cut(args[0], "/")
		if !found || owner == "" || repoName == "" {
			return nil, "", fmt.Errorf("expected owner/repo, got %q", args[0])
		}
		name = addName
		if name == "" {
			name = repoName
		}
		repo = config.RepositoryConfig{
			Type:      "github",
			Owner:     owner,
			Repo:      repoName,
			Token:     addToken,
			AssetName: addAsset,
		}

	default:
		return nil, "", fmt.Errorf("expected owner/repo or --url\n\nUsage: guppy add <owner>/<repo>")
	}

	if err := config.ValidateName(name); err != nil {
		return nil, "", err
	}
	if reserved(name) {
		return nil, "", fmt.Errorf("%q is a guppy command; use --name to pick a different app name", name)
	}

	a := config.New(name)
	a.Repository = repo
	a.Applier = addApplier
	a.Bin = addBin
	a.PreInstall = addPreInstall
	a.PostInstall = addPostInstall

	return a, name, nil
}

var removeYes bool

var removeCmd = &cobra.Command{
	Use:     "remove <app>",
	Aliases: []string{"rm"},
	Short:   "Remove an application and everything guppy installed for it",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := config.LoadApp(args[0])
		if err != nil {
			return err
		}

		installDir, err := config.InstallDir(a.Name())
		if err != nil {
			return err
		}

		if !removeYes {
			fmt.Printf("This will delete:\n  %s\n  %s\nand unlink %s from the bin directory.\n",
				a.Path(), installDir, strings.Join(a.Binaries(), ", "))
			fmt.Print("Continue? [y/N]: ")

			var answer string
			_, _ = fmt.Scanln(&answer)
			if !strings.EqualFold(strings.TrimSpace(answer), "y") {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := app.Remove(a); err != nil {
			return err
		}

		fmt.Printf("✓ Removed %s\n", a.Name())
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "directory holding guppy's app configs")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	rootCmd.Flags().StringVarP(&intervalFlag, "interval", "i", "", "check for updates at regular intervals (e.g., 15m, 1h, 1d, or HH:MM:SS)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if configDir != "" {
			return os.Setenv(config.EnvConfigDir, configDir)
		}
		return nil
	}

	addCmd.Flags().StringVar(&addName, "name", "", "name for the app (defaults to the repo name)")
	addCmd.Flags().StringVar(&addApplier, "applier", "binary", "how to install the download (binary or archive)")
	addCmd.Flags().StringArrayVar(&addBin, "bin", nil, "binary to link into the bin directory (repeatable)")
	addCmd.Flags().StringVar(&addAsset, "asset", "", "release asset name to download")
	addCmd.Flags().StringVar(&addToken, "token", "", "GitHub token for private repos or higher rate limits")
	addCmd.Flags().StringVar(&addURL, "url", "", "releases.json URL, for the http provider")
	addCmd.Flags().StringArrayVar(&addPreInstall, "pre-install", nil, "shell command to run before installing (repeatable)")
	addCmd.Flags().StringArrayVar(&addPostInstall, "post-install", nil, "shell command to run after installing (repeatable)")

	removeCmd.Flags().BoolVarP(&removeYes, "yes", "y", false, "don't ask for confirmation")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(binCmd)
}

// reserved reports whether a name would be shadowed by a guppy subcommand.
func reserved(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name || slices.Contains(cmd.Aliases, name) {
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
func forEachApp(names []string, fn func(*config.App) error) error {
	var failed int
	for _, name := range names {
		a, err := config.LoadApp(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed++
			continue
		}

		debugLog("Loaded app %s from %s", a.Name(), a.Path())
		if err := fn(a); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d app(s) failed", failed, len(names))
	}
	return nil
}

func newInstaller(a *config.App) (*app.Installer, error) {
	repo, err := app.NewRepository(a, debug)
	if err != nil {
		return nil, err
	}
	return &app.Installer{Repo: repo, Debug: debug}, nil
}

func updateApps(ctx context.Context, args []string) error {
	apps, err := resolveApps(args)
	if err != nil {
		return err
	}

	return forEachApp(apps, func(a *config.App) error {
		installer, err := newInstaller(a)
		if err != nil {
			return err
		}
		return installer.Install(ctx, a)
	})
}

// onPath reports whether dir is already in PATH.
func onPath(dir string) bool {
	return slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir)
}
