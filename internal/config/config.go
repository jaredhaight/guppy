package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Repository   RepositoryConfig `json:"repository" mapstructure:"repository"`
	CurrentVersion string         `json:"current_version" mapstructure:"current_version"`
	TargetPath   string           `json:"target_path" mapstructure:"target_path"`
	Applier      string           `json:"applier" mapstructure:"applier"`
	DownloadDir  string           `json:"download_dir" mapstructure:"download_dir"`
}

// RepositoryConfig represents repository configuration
type RepositoryConfig struct {
	Type      string `json:"type" mapstructure:"type"`
	Owner     string `json:"owner" mapstructure:"owner"`
	Repo      string `json:"repo" mapstructure:"repo"`
	Token     string `json:"token,omitempty" mapstructure:"token"`
	AssetName string `json:"asset_name,omitempty" mapstructure:"asset_name"`
}

// Load loads configuration from a JSON file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file details
	v.SetConfigType("json")

	if configPath != "" {
		// Use specified config file
		v.SetConfigFile(configPath)
	} else {
		// Look for config in common locations
		v.SetConfigName("guppy")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.config/guppy")
		v.AddConfigPath("/etc/guppy")
	}

	// Set defaults
	v.SetDefault("applier", "binary")
	v.SetDefault("download_dir", "/tmp/guppy")
	v.SetDefault("repository.type", "github")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Repository.Type == "" {
		return fmt.Errorf("repository type is required")
	}

	if c.Repository.Type == "github" {
		if c.Repository.Owner == "" {
			return fmt.Errorf("repository owner is required for GitHub")
		}
		if c.Repository.Repo == "" {
			return fmt.Errorf("repository repo is required for GitHub")
		}
	}

	if c.TargetPath == "" {
		return fmt.Errorf("target_path is required")
	}

	if c.Applier == "" {
		return fmt.Errorf("applier is required")
	}

	return nil
}

// Save saves the configuration to a JSON file
func (c *Config) Save(configPath string) error {
	v := viper.New()
	v.SetConfigType("json")

	// Set all config values
	v.Set("repository", c.Repository)
	v.Set("current_version", c.CurrentVersion)
	v.Set("target_path", c.TargetPath)
	v.Set("applier", c.Applier)
	v.Set("download_dir", c.DownloadDir)

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	// Write config file
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default config file path
// The config file should be in the same directory as the guppy executable
func GetDefaultConfigPath() string {
	// Get the executable path
	exePath, err := os.Executable()
	if err != nil {
		return "guppy.json"
	}

	// Return path to guppy.json in the executable directory
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "guppy.json")
}
