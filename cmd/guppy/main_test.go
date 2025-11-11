package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaredhaight/guppy/internal/config"
	"github.com/jaredhaight/guppy/pkg/repository"
)

// Mock repository for testing
type mockRepository struct {
	latestRelease      *repository.Release
	getLatestReleaseErr error
	compareVersionsResult bool
	compareVersionsErr error
	downloadErr        error
	downloadCalled     bool
}

func (m *mockRepository) GetLatestRelease() (*repository.Release, error) {
	return m.latestRelease, m.getLatestReleaseErr
}

func (m *mockRepository) GetRelease(version string) (*repository.Release, error) {
	return m.latestRelease, m.getLatestReleaseErr
}

func (m *mockRepository) CompareVersions(current, latest string) (bool, error) {
	return m.compareVersionsResult, m.compareVersionsErr
}

func (m *mockRepository) Download(release *repository.Release, destination string) error {
	m.downloadCalled = true
	if m.downloadErr != nil {
		return m.downloadErr
	}
	// Create a dummy file for testing
	return os.WriteFile(destination, []byte("mock download content"), 0644)
}

func (m *mockRepository) SetDebug(enabled bool) {}

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test config file
	configPath := filepath.Join(tempDir, "test-config.json")
	configContent := `{
  "repository": {
    "type": "github",
    "owner": "testowner",
    "repo": "testrepo"
  },
  "target_path": "/usr/local/bin/app",
  "applier": "binary"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set the config file path
	cfgFile = configPath

	err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("loadConfig() did not set global cfg variable")
	}

	if cfg.Repository.Type != "github" {
		t.Errorf("cfg.Repository.Type = %s, want github", cfg.Repository.Type)
	}
	if cfg.Repository.Owner != "testowner" {
		t.Errorf("cfg.Repository.Owner = %s, want testowner", cfg.Repository.Owner)
	}
	if cfg.Repository.Repo != "testrepo" {
		t.Errorf("cfg.Repository.Repo = %s, want testrepo", cfg.Repository.Repo)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	cfgFile = filepath.Join(tempDir, "nonexistent.json")

	err := loadConfig()
	if err == nil {
		t.Error("loadConfig() expected error for nonexistent file, got nil")
	}
}

func TestCreateRepository_GitHub(t *testing.T) {
	cfg = &config.Config{
		Repository: config.RepositoryConfig{
			Type:      "github",
			Owner:     "testowner",
			Repo:      "testrepo",
			Token:     "ghp_token123",
			AssetName: "app-linux",
		},
	}

	repo, err := createRepository()
	if err != nil {
		t.Fatalf("createRepository() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("createRepository() returned nil repository")
	}

	// Verify it's a GitHub repository by checking the type
	if _, ok := repo.(*repository.GitHubRepository); !ok {
		t.Errorf("createRepository() did not return GitHubRepository")
	}
}

func TestCreateRepository_HTTP(t *testing.T) {
	cfg = &config.Config{
		Repository: config.RepositoryConfig{
			Type: "http",
			URL:  "https://example.com/releases.json",
		},
	}

	repo, err := createRepository()
	if err != nil {
		t.Fatalf("createRepository() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("createRepository() returned nil repository")
	}

	// Verify it's an HTTP repository by checking the type
	if _, ok := repo.(*repository.HTTPRepository); !ok {
		t.Errorf("createRepository() did not return HTTPRepository")
	}
}

func TestCreateRepository_UnsupportedType(t *testing.T) {
	cfg = &config.Config{
		Repository: config.RepositoryConfig{
			Type: "unsupported",
		},
	}

	_, err := createRepository()
	if err == nil {
		t.Error("createRepository() expected error for unsupported type, got nil")
	}
}

func TestPerformUpdate_NoUpdateNeeded(t *testing.T) {
	tempDir := t.TempDir()

	cfg = &config.Config{
		CurrentVersion: "v2.0.0",
		DownloadDir:    tempDir,
		TargetPath:     filepath.Join(tempDir, "target"),
		Applier:        "binary",
	}
	cfgFile = filepath.Join(tempDir, "config.json")

	mockRepo := &mockRepository{
		latestRelease: &repository.Release{
			Version: "v2.0.0",
		},
		compareVersionsResult: false, // Not newer
	}

	err := performUpdate(mockRepo)
	if err != nil {
		t.Fatalf("performUpdate() failed: %v", err)
	}

	if mockRepo.downloadCalled {
		t.Error("performUpdate() should not download when already up to date")
	}
}

func TestPerformUpdate_NewVersionDownloadAndApply(t *testing.T) {
	tempDir := t.TempDir()

	// Create target file
	targetPath := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetPath, []byte("old version"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	configPath := filepath.Join(tempDir, "config.json")
	cfg = &config.Config{
		CurrentVersion: "v1.0.0",
		DownloadDir:    filepath.Join(tempDir, "downloads"),
		TargetPath:     targetPath,
		Applier:        "binary",
		Repository: config.RepositoryConfig{
			Type:  "github",
			Owner: "test",
			Repo:  "test",
		},
	}
	cfgFile = configPath

	// Save initial config
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	mockRepo := &mockRepository{
		latestRelease: &repository.Release{
			Version:  "v2.0.0",
			FileName: "app-v2.0.0.bin",
		},
		compareVersionsResult: true, // Is newer
	}

	err := performUpdate(mockRepo)
	if err != nil {
		t.Fatalf("performUpdate() failed: %v", err)
	}

	if !mockRepo.downloadCalled {
		t.Error("performUpdate() should have called Download()")
	}

	// Verify config was updated with new version
	updatedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	if updatedCfg.CurrentVersion != "v2.0.0" {
		t.Errorf("Config current_version = %s, want v2.0.0", updatedCfg.CurrentVersion)
	}
}

func TestPerformUpdate_DownloadError(t *testing.T) {
	tempDir := t.TempDir()

	cfg = &config.Config{
		CurrentVersion: "v1.0.0",
		DownloadDir:    tempDir,
		TargetPath:     filepath.Join(tempDir, "target"),
		Applier:        "binary",
	}
	cfgFile = filepath.Join(tempDir, "config.json")

	mockRepo := &mockRepository{
		latestRelease: &repository.Release{
			Version:  "v2.0.0",
			FileName: "app.bin",
		},
		compareVersionsResult: true,
		downloadErr:           os.ErrPermission,
	}

	err := performUpdate(mockRepo)
	if err == nil {
		t.Error("performUpdate() expected error when download fails, got nil")
	}
}

func TestPerformUpdate_UnknownApplierType(t *testing.T) {
	tempDir := t.TempDir()

	// Create download directory and file
	downloadDir := filepath.Join(tempDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		t.Fatalf("Failed to create download dir: %v", err)
	}

	cfg = &config.Config{
		CurrentVersion: "v1.0.0",
		DownloadDir:    downloadDir,
		TargetPath:     filepath.Join(tempDir, "target"),
		Applier:        "unknown_applier",
	}
	cfgFile = filepath.Join(tempDir, "config.json")

	mockRepo := &mockRepository{
		latestRelease: &repository.Release{
			Version:  "v2.0.0",
			FileName: "app.bin",
		},
		compareVersionsResult: true,
	}

	err := performUpdate(mockRepo)
	if err == nil {
		t.Error("performUpdate() expected error for unknown applier type, got nil")
	}
}

func TestPerformUpdate_NoCurrentVersion(t *testing.T) {
	tempDir := t.TempDir()

	targetPath := filepath.Join(tempDir, "target")
	configPath := filepath.Join(tempDir, "config.json")

	cfg = &config.Config{
		CurrentVersion: "", // No current version
		DownloadDir:    filepath.Join(tempDir, "downloads"),
		TargetPath:     targetPath,
		Applier:        "binary",
		Repository: config.RepositoryConfig{
			Type:  "github",
			Owner: "test",
			Repo:  "test",
		},
	}
	cfgFile = configPath

	// Save config
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	mockRepo := &mockRepository{
		latestRelease: &repository.Release{
			Version:  "v1.0.0",
			FileName: "app.bin",
		},
	}

	err := performUpdate(mockRepo)
	if err != nil {
		t.Fatalf("performUpdate() failed when no current version: %v", err)
	}

	// Should proceed with download when no current version is set
	if !mockRepo.downloadCalled {
		t.Error("performUpdate() should download when no current version is set")
	}
}

func TestDebugLog(t *testing.T) {
	// Save original stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Test with debug enabled
	debug = true
	debugLog("test message: %s", "hello")

	// Test with debug disabled
	debug = false
	debugLog("should not appear")

	// Restore stderr
	_ = w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("[DEBUG] test message: hello")) {
		t.Error("debugLog() should output message when debug is enabled")
	}
	if bytes.Contains([]byte(output), []byte("should not appear")) {
		t.Error("debugLog() should not output message when debug is disabled")
	}

	// Reset debug flag
	debug = false
}

func TestVersionCmd(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set a test version
	oldVersion := Version
	Version = "v1.2.3-test"

	// Execute version command
	versionCmd.Run(versionCmd, []string{})

	// Restore stdout and version
	_ = w.Close()
	os.Stdout = oldStdout
	Version = oldVersion

	// Read captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("v1.2.3-test")) {
		t.Error("Version command should output the version number")
	}
	if !bytes.Contains([]byte(output), []byte("Guppy Software Updater")) {
		t.Error("Version command should output the program name")
	}
}

func TestCreateRepository_WithAssetName(t *testing.T) {
	cfg = &config.Config{
		Repository: config.RepositoryConfig{
			Type:      "github",
			Owner:     "testowner",
			Repo:      "testrepo",
			AssetName: "specific-asset",
		},
	}

	repo, err := createRepository()
	if err != nil {
		t.Fatalf("createRepository() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("createRepository() returned nil repository")
	}

	// The asset name should be set on the repository
	githubRepo, ok := repo.(*repository.GitHubRepository)
	if !ok {
		t.Fatal("Expected GitHubRepository")
	}

	// We can't easily verify the asset name was set without exposing it,
	// but we can verify the repository was created successfully
	if githubRepo == nil {
		t.Error("GitHub repository should not be nil")
	}
}

func TestCreateRepository_WithDebug(t *testing.T) {
	debug = true
	defer func() { debug = false }()

	cfg = &config.Config{
		Repository: config.RepositoryConfig{
			Type:  "github",
			Owner: "testowner",
			Repo:  "testrepo",
		},
	}

	repo, err := createRepository()
	if err != nil {
		t.Fatalf("createRepository() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("createRepository() returned nil repository")
	}

	// Debug should be set on the repository (we can't easily verify this without
	// exposing the debug flag, but we can verify creation succeeded)
}
