package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/jaredhaight/guppy/pkg/repository"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Extensions accepted for app config files, in the order they're searched.
// The first entry is what "guppy add" writes.
var Extensions = []string{".yaml", ".yml", ".json"}

// App is the configuration for a single managed application. It lives in its
// own file under <config>/apps/, and the app's name comes from the filename.
type App struct {
	Repository     RepositoryConfig `json:"repository" yaml:"repository" mapstructure:"repository"`
	CurrentVersion string           `json:"current_version" yaml:"current_version" mapstructure:"current_version"`
	Applier        string           `json:"applier" yaml:"applier" mapstructure:"applier"`

	// Bin lists the binaries to link into the bin directory. Each entry is
	// resolved against the install directory; see ResolveBin. Defaults to the
	// app's own name.
	Bin []string `json:"bin,omitempty" yaml:"bin,omitempty" mapstructure:"bin"`

	// PreInstall and PostInstall are shell commands run before and after the
	// download is applied.
	PreInstall  []string `json:"pre_install,omitempty" yaml:"pre_install,omitempty" mapstructure:"pre_install"`
	PostInstall []string `json:"post_install,omitempty" yaml:"post_install,omitempty" mapstructure:"post_install"`

	// AllowUnverified drops guppy's integrity requirements for this app: it
	// permits plain-HTTP URLs and releases that ship no checksum. Both mean
	// guppy cannot tell a genuine release from a substituted one, so this is
	// opt-in per app and never a default.
	AllowUnverified bool `json:"allow_unverified,omitempty" yaml:"allow_unverified,omitempty" mapstructure:"allow_unverified"`

	name string // from the filename
	path string // where it was loaded from, so Save round-trips the same format
}

// RepositoryConfig represents repository configuration
type RepositoryConfig struct {
	Type      string `json:"type" yaml:"type" mapstructure:"type"`
	Owner     string `json:"owner,omitempty" yaml:"owner,omitempty" mapstructure:"owner"`
	Repo      string `json:"repo,omitempty" yaml:"repo,omitempty" mapstructure:"repo"`
	Token     string `json:"token,omitempty" yaml:"token,omitempty" mapstructure:"token"`
	AssetName string `json:"asset_name,omitempty" yaml:"asset_name,omitempty" mapstructure:"asset_name"`
	URL       string `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url"`
}

// App names become filenames and directory names, so they're restricted rather
// than sanitized: anything outside this set is rejected up front.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateName reports whether name is usable as an app name.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("app name is required")
	}
	if !validName.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid app name %q: use letters, numbers, dot, dash and underscore", name)
	}
	return nil
}

// New returns an empty app config with the given name.
func New(name string) *App {
	return &App{name: name, Applier: "binary"}
}

// Name returns the app's name.
func (a *App) Name() string { return a.name }

// Path returns the file the app was loaded from, or "" if it has never been
// written.
func (a *App) Path() string { return a.path }

// AppPath returns the path of the config file for name, whichever accepted
// extension it uses. It returns an error if more than one file claims the name.
func AppPath(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	dir, err := AppsDir()
	if err != nil {
		return "", err
	}

	var found []string
	for _, ext := range Extensions {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("no app named %q (looked in %s)", name, dir)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("multiple config files for app %q: %s", name, strings.Join(found, ", "))
	}
}

// ListApps returns the names of all configured apps, sorted.
func ListApps() ([]string, error) {
	dir, err := AppsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading apps directory: %w", err)
	}

	seen := make(map[string]bool)
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if !slices.Contains(Extensions, ext) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	slices.Sort(names)
	return names, nil
}

// LoadApp loads the config for a named app.
func LoadApp(name string) (*App, error) {
	path, err := AppPath(name)
	if err != nil {
		return nil, err
	}
	return LoadFile(name, path)
}

// LoadFile loads an app config from an explicit path. The format is inferred
// from the file extension.
func LoadFile(name, path string) (*App, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("applier", "binary")
	v.SetDefault("repository.type", "github")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	var app App
	// UnmarshalExact rejects keys that don't map to a field, which catches
	// typos in hand-written config regardless of format.
	if err := v.UnmarshalExact(&app); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}

	app.name = name
	app.path = path

	if err := app.Validate(); err != nil {
		return nil, fmt.Errorf("error in %s: %w", path, err)
	}

	return &app, nil
}

// Validate validates the configuration
func (a *App) Validate() error {
	if err := ValidateName(a.name); err != nil {
		return err
	}

	if a.Repository.Type == "" {
		return fmt.Errorf("repository type is required")
	}

	if a.Repository.Type != "github" && a.Repository.Type != "http" {
		return fmt.Errorf("invalid repository type: %s (valid values: github, http)", a.Repository.Type)
	}

	if a.Repository.Type == "github" {
		if a.Repository.Owner == "" {
			return fmt.Errorf("repository owner is required for GitHub")
		}
		if a.Repository.Repo == "" {
			return fmt.Errorf("repository repo is required for GitHub")
		}
	}

	if a.Repository.Type == "http" {
		if a.Repository.URL == "" {
			return fmt.Errorf("repository url is required for HTTP")
		}
		if !a.AllowUnverified {
			if err := repository.ValidateURL(a.Repository.URL); err != nil {
				return err
			}
		}
	}

	if a.Applier == "" {
		return fmt.Errorf("applier is required")
	}

	if a.Applier != "binary" && a.Applier != "archive" {
		return fmt.Errorf("invalid applier type: %s (valid values: binary, archive)", a.Applier)
	}

	// The binary applier installs the one downloaded file, so it has exactly
	// one binary to offer.
	if a.Applier == "binary" && len(a.Bin) > 1 {
		return fmt.Errorf("the binary applier installs a single file, but bin lists %d entries", len(a.Bin))
	}

	return nil
}

// Binaries returns the bin entries to link, defaulting to the app's own name.
func (a *App) Binaries() []string {
	if len(a.Bin) > 0 {
		return a.Bin
	}
	return []string{a.name}
}

// Save writes the app config back to the file it was loaded from, creating it
// as YAML under the apps directory if it has never been written.
func (a *App) Save() error {
	if a.path == "" {
		dir, err := AppsDir()
		if err != nil {
			return err
		}
		if err := ValidateName(a.name); err != nil {
			return err
		}
		a.path = filepath.Join(dir, a.name+Extensions[0])
	}

	var (
		data []byte
		err  error
	)
	if filepath.Ext(a.path) == ".json" {
		data, err = json.MarshalIndent(a, "", "  ")
		data = append(data, '\n')
	} else {
		data, err = yaml.Marshal(a)
	}
	if err != nil {
		return fmt.Errorf("error encoding config: %w", err)
	}

	// Owner-only: these files hold repository.token, a GitHub PAT.
	if err := os.MkdirAll(filepath.Dir(a.path), 0700); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	if err := os.WriteFile(a.path, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	// WriteFile only applies the mode when it creates the file, so configs
	// written by an older guppy would keep their 0644 forever without this.
	if err := os.Chmod(a.path, 0600); err != nil {
		return fmt.Errorf("error securing config file: %w", err)
	}

	return nil
}

// RemoveApp deletes an app's config file.
func RemoveApp(name string) error {
	path, err := AppPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("error removing config file: %w", err)
	}
	return nil
}
