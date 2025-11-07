package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jaredhaight/guppy/internal/config"
	"github.com/jaredhaight/guppy/pkg/applier"
	"github.com/jaredhaight/guppy/pkg/checksum"
	"github.com/jaredhaight/guppy/pkg/repository"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	cfgFile   string
	cfg       *config.Config
	debug     bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// debugLog prints a debug message if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

var rootCmd = &cobra.Command{
	Use:          "guppy",
	Short:        "Guppy is a software update helper",
	Long:         `Guppy checks for new releases, downloads them, and applies the new version.`,
	SilenceUsage: true,
	SilenceErrors: true,
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for available updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfig(); err != nil {
			return err
		}

		repo, err := createRepository()
		if err != nil {
			return err
		}

		fmt.Println("Checking for updates...")
		latest, err := repo.GetLatestRelease()
		if err != nil {
			return fmt.Errorf("error getting latest release: %w", err)
		}

		fmt.Printf("Latest version: %s\n", latest.Version)

		if cfg.CurrentVersion == "" {
			fmt.Println("No current version set in config")
			return nil
		}

		fmt.Printf("Current version: %s\n", cfg.CurrentVersion)

		isNewer, err := repo.CompareVersions(cfg.CurrentVersion, latest.Version)
		if err != nil {
			return fmt.Errorf("error comparing versions: %w", err)
		}

		if isNewer {
			fmt.Printf("🎉 New version available: %s\n", latest.Version)
			fmt.Printf("Download URL: %s\n", latest.DownloadURL)
		} else {
			fmt.Println("✓ You are up to date!")
		}

		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and apply updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfig(); err != nil {
			return err
		}

		repo, err := createRepository()
		if err != nil {
			return err
		}

		fmt.Println("Checking for updates...")
		latest, err := repo.GetLatestRelease()
		if err != nil {
			return fmt.Errorf("error getting latest release: %w", err)
		}

		if cfg.CurrentVersion != "" {
			isNewer, err := repo.CompareVersions(cfg.CurrentVersion, latest.Version)
			if err != nil {
				return fmt.Errorf("error comparing versions: %w", err)
			}

			if !isNewer {
				fmt.Println("✓ Already up to date!")
				return nil
			}
		}

		fmt.Printf("Downloading version %s...\n", latest.Version)

		// Create download directory
		if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
			return fmt.Errorf("error creating download directory: %w", err)
		}

		downloadPath := filepath.Join(cfg.DownloadDir, latest.FileName)
		debugLog("Computed download path: %s", downloadPath)
		if err := repo.Download(latest, downloadPath); err != nil {
			return fmt.Errorf("error downloading release: %w", err)
		}

		fmt.Printf("Downloaded to: %s\n", downloadPath)

		// Verify checksum if provided
		if latest.Checksum != "" {
			fmt.Println("Verifying checksum...")
			valid, err := checksum.VerifySHA256(downloadPath, latest.Checksum)
			if err != nil {
				return fmt.Errorf("error verifying checksum: %w", err)
			}
			if !valid {
				return fmt.Errorf("checksum verification failed - file may be corrupted")
			}
			fmt.Println("✓ Checksum verified")
		}

		// Apply the update
		fmt.Printf("Applying update to %s...\n", cfg.TargetPath)

		var app applier.Applier
		switch cfg.Applier {
		case "binary":
			app = applier.NewBinaryApplier()
		case "archive":
			app = applier.NewArchiveApplier()
		default:
			return fmt.Errorf("unknown applier type: %s", cfg.Applier)
		}

		if err := app.Apply(downloadPath, cfg.TargetPath); err != nil {
			return fmt.Errorf("error applying update: %w", err)
		}

		fmt.Println("✓ Update applied successfully!")

		// Update current version in config
		cfg.CurrentVersion = latest.Version
		if err := cfg.Save(cfgFile); err != nil {
			fmt.Printf("Warning: Could not save updated version to config: %v\n", err)
		}

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show guppy version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("guppy version %s\n", Version)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a template configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := cfgFile
		if configPath == "" {
			configPath = config.GetDefaultConfigPath()
		}

		// Check if config file already exists
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s", configPath)
		}

		// Create a template config with example values
		templateConfig := &config.Config{
			Repository: config.RepositoryConfig{
				Type:      "github",
				Owner:     "owner",
				Repo:      "repo",
				Token:     "",
				AssetName: "",
			},
			CurrentVersion: "",
			TargetPath:     "/path/to/target/binary",
			Applier:        "binary",
			DownloadDir:    "/tmp/guppy",
		}

		// Save the template config
		if err := templateConfig.Save(configPath); err != nil {
			return fmt.Errorf("error creating config file: %w", err)
		}

		fmt.Printf("✓ Created template config file at: %s\n", configPath)
		fmt.Println("\nPlease edit the config file and update the following fields:")
		fmt.Println("  - repository.owner: GitHub repository owner")
		fmt.Println("  - repository.repo: GitHub repository name")
		fmt.Println("  - target_path: Path where the binary should be installed")
		fmt.Println("\nOptional fields:")
		fmt.Println("  - repository.token: GitHub personal access token (for private repos or higher rate limits)")
		fmt.Println("  - repository.asset_name: Specific asset name pattern to download")
		fmt.Println("  - current_version: Current version (will be auto-updated after first update)")
		fmt.Println("  - applier: Type of applier (binary or archive)")
		fmt.Println("  - download_dir: Directory for temporary downloads")

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is guppy.json in executable directory)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
}

func loadConfig() error {
	var err error
	if cfgFile == "" {
		cfgFile = config.GetDefaultConfigPath()
	}
	debugLog("Loading config from: %s", cfgFile)
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("%w\n\nYou can specify a config file location using the --config flag.\nTo create a template config file, run: guppy init --config <path>", err)
	}
	return nil
}

func createRepository() (repository.Repository, error) {
	switch cfg.Repository.Type {
	case "github":
		repo := repository.NewGitHubRepository(
			cfg.Repository.Owner,
			cfg.Repository.Repo,
			cfg.Repository.Token,
		)
		if cfg.Repository.AssetName != "" {
			repo.SetAssetName(cfg.Repository.AssetName)
		}
		repo.SetDebug(debug)
		return repo, nil
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", cfg.Repository.Type)
	}
}
