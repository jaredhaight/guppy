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
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "guppy",
	Short: "Guppy is a software update helper",
	Long:  `Guppy checks for new releases, downloads them, and applies the new version.`,
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

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.config/guppy/guppy.json)")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
}

func loadConfig() error {
	var err error
	if cfgFile == "" {
		cfgFile = config.GetDefaultConfigPath()
	}
	cfg, err = config.Load(cfgFile)
	return err
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
		return repo, nil
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", cfg.Repository.Type)
	}
}
