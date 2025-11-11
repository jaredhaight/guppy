package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_GitHubConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid GitHub config
	configPath := filepath.Join(tempDir, "guppy.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "testowner",
    "repo": "testrepo",
    "token": "ghp_testtoken123",
    "asset_name": "app-linux-amd64"
  },
  "current_version": "v1.0.0",
  "target_path": "/usr/local/bin/app",
  "applier": "binary",
  "download_dir": "/tmp/downloads"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if config.Repository.Type != "github" {
		t.Errorf("Repository.Type = %s, want github", config.Repository.Type)
	}
	if config.Repository.Owner != "testowner" {
		t.Errorf("Repository.Owner = %s, want testowner", config.Repository.Owner)
	}
	if config.Repository.Repo != "testrepo" {
		t.Errorf("Repository.Repo = %s, want testrepo", config.Repository.Repo)
	}
	if config.Repository.Token != "ghp_testtoken123" {
		t.Errorf("Repository.Token = %s, want ghp_testtoken123", config.Repository.Token)
	}
	if config.Repository.AssetName != "app-linux-amd64" {
		t.Errorf("Repository.AssetName = %s, want app-linux-amd64", config.Repository.AssetName)
	}
	if config.CurrentVersion != "v1.0.0" {
		t.Errorf("CurrentVersion = %s, want v1.0.0", config.CurrentVersion)
	}
	if config.TargetPath != "/usr/local/bin/app" {
		t.Errorf("TargetPath = %s, want /usr/local/bin/app", config.TargetPath)
	}
	if config.Applier != "binary" {
		t.Errorf("Applier = %s, want binary", config.Applier)
	}
	if config.DownloadDir != "/tmp/downloads" {
		t.Errorf("DownloadDir = %s, want /tmp/downloads", config.DownloadDir)
	}
}

func TestLoad_HTTPConfig(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "guppy.json")
	configContent := `{
  "repository": {
    "type": "http",
    "url": "https://example.com/releases"
  },
  "current_version": "v2.0.0",
  "target_path": "/opt/myapp/bin/app",
  "applier": "archive"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if config.Repository.Type != "http" {
		t.Errorf("Repository.Type = %s, want http", config.Repository.Type)
	}
	if config.Repository.URL != "https://example.com/releases" {
		t.Errorf("Repository.URL = %s, want https://example.com/releases", config.Repository.URL)
	}
	if config.CurrentVersion != "v2.0.0" {
		t.Errorf("CurrentVersion = %s, want v2.0.0", config.CurrentVersion)
	}
	if config.Applier != "archive" {
		t.Errorf("Applier = %s, want archive", config.Applier)
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "guppy.json")
	// Config with minimal required fields, should apply defaults
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "testowner",
    "repo": "testrepo"
  },
  "target_path": "/usr/local/bin/app"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check defaults were applied
	if config.Applier != "binary" {
		t.Errorf("Applier = %s, want binary (default)", config.Applier)
	}
	if config.DownloadDir == "" {
		t.Error("DownloadDir should have default value")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	nonexistentPath := filepath.Join(tempDir, "nonexistent.json")

	_, err := Load(nonexistentPath)
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "invalid.json")
	invalidContent := `{ "repository": { "type": "github" }`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
}

func TestLoad_UnknownTopLevelKey(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "unknown-key.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "test",
    "repo": "test"
  },
  "target_path": "/usr/local/bin/app",
  "unknown_field": "should cause error"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for unknown key, got nil")
	}
}

func TestLoad_UnknownRepositoryKey(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "unknown-repo-key.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "test",
    "repo": "test",
    "unknown_repo_field": "should cause error"
  },
  "target_path": "/usr/local/bin/app"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for unknown repository key, got nil")
	}
}

func TestValidate_ValidGitHubConfig(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
			Repo:  "testrepo",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validate() failed for valid GitHub config: %v", err)
	}
}

func TestValidate_ValidHTTPConfig(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type: "http",
			URL:  "https://example.com/releases",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "archive",
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validate() failed for valid HTTP config: %v", err)
	}
}

func TestValidate_MissingRepositoryType(t *testing.T) {
	config := &Config{
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing repository type, got nil")
	}
}

func TestValidate_InvalidRepositoryType(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type: "invalid",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid repository type, got nil")
	}
}

func TestValidate_GitHubMissingOwner(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type: "github",
			Repo: "testrepo",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing GitHub owner, got nil")
	}
}

func TestValidate_GitHubMissingRepo(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing GitHub repo, got nil")
	}
}

func TestValidate_HTTPMissingURL(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type: "http",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing HTTP URL, got nil")
	}
}

func TestValidate_MissingTargetPath(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
			Repo:  "testrepo",
		},
		Applier: "binary",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing target path, got nil")
	}
}

func TestValidate_MissingApplier(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
			Repo:  "testrepo",
		},
		TargetPath: "/usr/local/bin/app",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing applier, got nil")
	}
}

func TestValidate_InvalidApplierType(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
			Repo:  "testrepo",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "invalid",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid applier type, got nil")
	}
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()

	config := &Config{
		Repository: RepositoryConfig{
			Type:      "github",
			Owner:     "testowner",
			Repo:      "testrepo",
			Token:     "ghp_token123",
			AssetName: "app-linux",
		},
		CurrentVersion: "v1.2.3",
		TargetPath:     "/usr/local/bin/app",
		Applier:        "binary",
		DownloadDir:    "/tmp/guppy",
	}

	configPath := filepath.Join(tempDir, "saved-config.json")
	err := config.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Save() did not create config file")
	}

	// Load the saved config and verify
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Repository.Type != config.Repository.Type {
		t.Errorf("Saved Repository.Type = %s, want %s", loadedConfig.Repository.Type, config.Repository.Type)
	}
	if loadedConfig.Repository.Owner != config.Repository.Owner {
		t.Errorf("Saved Repository.Owner = %s, want %s", loadedConfig.Repository.Owner, config.Repository.Owner)
	}
	if loadedConfig.Repository.Repo != config.Repository.Repo {
		t.Errorf("Saved Repository.Repo = %s, want %s", loadedConfig.Repository.Repo, config.Repository.Repo)
	}
	if loadedConfig.CurrentVersion != config.CurrentVersion {
		t.Errorf("Saved CurrentVersion = %s, want %s", loadedConfig.CurrentVersion, config.CurrentVersion)
	}
	if loadedConfig.TargetPath != config.TargetPath {
		t.Errorf("Saved TargetPath = %s, want %s", loadedConfig.TargetPath, config.TargetPath)
	}
	if loadedConfig.Applier != config.Applier {
		t.Errorf("Saved Applier = %s, want %s", loadedConfig.Applier, config.Applier)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()

	config := &Config{
		Repository: RepositoryConfig{
			Type:  "github",
			Owner: "test",
			Repo:  "test",
		},
		TargetPath: "/usr/local/bin/app",
		Applier:    "binary",
	}

	// Save to a nested directory that doesn't exist
	configPath := filepath.Join(tempDir, "nested", "dir", "config.json")
	err := config.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Save() did not create parent directory")
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Save() did not create config file")
	}
}

func TestSave_HTTPConfig(t *testing.T) {
	tempDir := t.TempDir()

	config := &Config{
		Repository: RepositoryConfig{
			Type: "http",
			URL:  "https://example.com/releases",
		},
		CurrentVersion: "v3.0.0",
		TargetPath:     "/opt/app/bin",
		Applier:        "archive",
		DownloadDir:    "/var/tmp/downloads",
	}

	configPath := filepath.Join(tempDir, "http-config.json")
	err := config.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load and verify
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved HTTP config: %v", err)
	}

	if loadedConfig.Repository.Type != "http" {
		t.Errorf("Saved Repository.Type = %s, want http", loadedConfig.Repository.Type)
	}
	if loadedConfig.Repository.URL != config.Repository.URL {
		t.Errorf("Saved Repository.URL = %s, want %s", loadedConfig.Repository.URL, config.Repository.URL)
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()

	if path == "" {
		t.Error("GetDefaultConfigPath() returned empty string")
	}

	// Should end with guppy.json
	if filepath.Base(path) != "guppy.json" {
		t.Errorf("GetDefaultConfigPath() = %s, should end with guppy.json", path)
	}
}

func TestValidateConfigKeys_ValidConfig(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "valid.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "test",
    "repo": "test"
  },
  "current_version": "v1.0.0",
  "target_path": "/usr/local/bin/app",
  "applier": "binary",
  "download_dir": "/tmp"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err := validateConfigKeys(configPath)
	if err != nil {
		t.Errorf("validateConfigKeys() failed for valid config: %v", err)
	}
}

func TestValidateConfigKeys_UnknownTopLevel(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "unknown.json")
	configContent := `{
  "repository": {
    "type": "github"
  },
  "target_path": "/usr/local/bin/app",
  "unknown_key": "value"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err := validateConfigKeys(configPath)
	if err == nil {
		t.Error("validateConfigKeys() expected error for unknown top-level key, got nil")
	}
}

func TestValidateConfigKeys_UnknownRepoKey(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "unknown-repo.json")
	configContent := `{
  "repository": {
    "type": "github",
    "unknown_repo_key": "value"
  },
  "target_path": "/usr/local/bin/app"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err := validateConfigKeys(configPath)
	if err == nil {
		t.Error("validateConfigKeys() expected error for unknown repository key, got nil")
	}
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "empty.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for empty config (missing required fields), got nil")
	}
}

func TestLoad_AllRepositoryFields(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "all-fields.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "testowner",
    "repo": "testrepo",
    "token": "ghp_token",
    "asset_name": "app-linux",
    "url": "https://example.com"
  },
  "target_path": "/usr/local/bin/app",
  "applier": "binary"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify all repository fields were loaded
	if config.Repository.Type != "github" {
		t.Errorf("Repository.Type = %s, want github", config.Repository.Type)
	}
	if config.Repository.Owner != "testowner" {
		t.Errorf("Repository.Owner = %s, want testowner", config.Repository.Owner)
	}
	if config.Repository.Repo != "testrepo" {
		t.Errorf("Repository.Repo = %s, want testrepo", config.Repository.Repo)
	}
	if config.Repository.Token != "ghp_token" {
		t.Errorf("Repository.Token = %s, want ghp_token", config.Repository.Token)
	}
	if config.Repository.AssetName != "app-linux" {
		t.Errorf("Repository.AssetName = %s, want app-linux", config.Repository.AssetName)
	}
	if config.Repository.URL != "https://example.com" {
		t.Errorf("Repository.URL = %s, want https://example.com", config.Repository.URL)
	}
}
