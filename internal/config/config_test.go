package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testRoot points the config and data directories at a temp dir so tests never
// touch the real home directory.
func testRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(EnvConfigDir, filepath.Join(root, "config"))
	t.Setenv(EnvDataDir, filepath.Join(root, "data"))
	return root
}

// writeApp drops a config file into the apps directory.
func writeApp(t *testing.T, filename, body string) string {
	t.Helper()
	dir, err := AppsDir()
	if err != nil {
		t.Fatalf("AppsDir() error: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create apps dir: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
	return path
}

const githubYAML = `repository:
  type: github
  owner: BurntSushi
  repo: ripgrep
  asset_name: ripgrep-.*-apple-darwin.tar.gz
current_version: 14.1.1
applier: archive
bin:
  - rg
pre_install:
  - echo before
post_install:
  - echo after
`

const githubJSON = `{
  "repository": {
    "type": "github",
    "owner": "BurntSushi",
    "repo": "ripgrep",
    "asset_name": "ripgrep-.*-apple-darwin.tar.gz"
  },
  "current_version": "14.1.1",
  "applier": "archive",
  "bin": ["rg"],
  "pre_install": ["echo before"],
  "post_install": ["echo after"]
}`

// YAML, YML and JSON must all produce the same config.
func TestLoadAppFormatsAgree(t *testing.T) {
	testRoot(t)

	writeApp(t, "yamlapp.yaml", githubYAML)
	writeApp(t, "ymlapp.yml", githubYAML)
	writeApp(t, "jsonapp.json", githubJSON)

	for _, name := range []string{"yamlapp", "ymlapp", "jsonapp"} {
		t.Run(name, func(t *testing.T) {
			a, err := LoadApp(name)
			if err != nil {
				t.Fatalf("LoadApp(%q) error: %v", name, err)
			}

			if a.Name() != name {
				t.Errorf("Name() = %q, want %q", a.Name(), name)
			}
			if a.Repository.Type != "github" || a.Repository.Owner != "BurntSushi" || a.Repository.Repo != "ripgrep" {
				t.Errorf("repository = %+v", a.Repository)
			}
			if a.Repository.AssetName != "ripgrep-.*-apple-darwin.tar.gz" {
				t.Errorf("AssetName = %q", a.Repository.AssetName)
			}
			if a.CurrentVersion != "14.1.1" {
				t.Errorf("CurrentVersion = %q, want 14.1.1", a.CurrentVersion)
			}
			if a.Applier != "archive" {
				t.Errorf("Applier = %q, want archive", a.Applier)
			}
			if len(a.Bin) != 1 || a.Bin[0] != "rg" {
				t.Errorf("Bin = %v, want [rg]", a.Bin)
			}
			if len(a.PreInstall) != 1 || a.PreInstall[0] != "echo before" {
				t.Errorf("PreInstall = %v", a.PreInstall)
			}
			if len(a.PostInstall) != 1 || a.PostInstall[0] != "echo after" {
				t.Errorf("PostInstall = %v", a.PostInstall)
			}
		})
	}
}

func TestLoadAppDefaults(t *testing.T) {
	testRoot(t)
	writeApp(t, "minimal.yaml", "repository:\n  owner: o\n  repo: r\n")

	a, err := LoadApp("minimal")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}

	if a.Repository.Type != "github" {
		t.Errorf("default repository.type = %q, want github", a.Repository.Type)
	}
	if a.Applier != "binary" {
		t.Errorf("default applier = %q, want binary", a.Applier)
	}
	// Bin defaults to the app's own name.
	if got := a.Binaries(); len(got) != 1 || got[0] != "minimal" {
		t.Errorf("Binaries() = %v, want [minimal]", got)
	}
}

// A config written for the removed http provider has to fail with an
// explanation. The url field is kept on RepositoryConfig precisely so this
// reaches Validate instead of dying in UnmarshalExact as an unknown key.
func TestLoadAppRejectsHTTPProviderConfig(t *testing.T) {
	testRoot(t)

	writeApp(t, "legacy.yaml", "repository:\n  type: http\n  url: https://example.com/releases.json\napplier: archive\n")

	_, err := LoadApp("legacy")
	if err == nil {
		t.Fatal("LoadApp() accepted an http provider config, want an error")
	}
	if !strings.Contains(err.Error(), "GitHub") {
		t.Errorf("error = %v, want it to explain that only GitHub is supported", err)
	}
	if strings.Contains(err.Error(), "invalid keys") {
		t.Errorf("error = %v, want the explanation rather than viper's unknown-key error", err)
	}
}

func TestLoadAppRejectsUnknownKeys(t *testing.T) {
	testRoot(t)

	tests := []struct {
		name, file, body string
	}{
		{
			name: "unknown top-level key in yaml",
			file: "a.yaml",
			body: "repository:\n  owner: o\n  repo: r\ntarget_path: /old/field\n",
		},
		{
			name: "unknown nested key in yaml",
			file: "b.yaml",
			body: "repository:\n  owner: o\n  repo: r\n  bogus: 1\n",
		},
		{
			name: "unknown top-level key in json",
			file: "c.json",
			body: `{"repository":{"owner":"o","repo":"r"},"download_dir":"/tmp"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeApp(t, tt.file, tt.body)
			name := tt.file[:len(tt.file)-len(filepath.Ext(tt.file))]
			if _, err := LoadFile(name, path); err == nil {
				t.Error("LoadFile() accepted an unknown key, want error")
			}
		})
	}
}

func TestSaveRoundTripsFormat(t *testing.T) {
	testRoot(t)

	cases := []struct{ name, file, body string }{
		{"yaml", "y.yaml", githubYAML},
		{"json", "j.json", githubJSON},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeApp(t, tc.file, tc.body)

			a, err := LoadFile(tc.name, path)
			if err != nil {
				t.Fatalf("LoadFile() error: %v", err)
			}

			a.CurrentVersion = "15.0.0"
			if err := a.Save(); err != nil {
				t.Fatalf("Save() error: %v", err)
			}

			// Saved in place, so the extension (and therefore format) is unchanged.
			if a.Path() != path {
				t.Errorf("Save() wrote to %q, want %q", a.Path(), path)
			}

			reloaded, err := LoadFile(tc.name, path)
			if err != nil {
				t.Fatalf("reload error: %v", err)
			}
			if reloaded.CurrentVersion != "15.0.0" {
				t.Errorf("CurrentVersion = %q, want 15.0.0", reloaded.CurrentVersion)
			}
			if len(reloaded.PostInstall) != 1 || reloaded.PostInstall[0] != "echo after" {
				t.Errorf("PostInstall lost in round-trip: %v", reloaded.PostInstall)
			}
			if len(reloaded.Bin) != 1 || reloaded.Bin[0] != "rg" {
				t.Errorf("Bin lost in round-trip: %v", reloaded.Bin)
			}
		})
	}
}

func TestSaveNewAppWritesYAML(t *testing.T) {
	testRoot(t)

	a := New("newapp")
	a.Repository = RepositoryConfig{Type: "github", Owner: "o", Repo: "r"}
	if err := a.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if filepath.Ext(a.Path()) != ".yaml" {
		t.Errorf("Save() wrote %q, want a .yaml file", a.Path())
	}
	if _, err := os.Stat(a.Path()); err != nil {
		t.Errorf("Save() did not create the file: %v", err)
	}
}

// Config files hold repository.token, a GitHub PAT, so they must not be
// readable by other users on the machine.
func TestSaveWritesOwnerOnlyPermissions(t *testing.T) {
	testRoot(t)

	a := New("secretapp")
	a.Repository = RepositoryConfig{Type: "github", Owner: "o", Repo: "r", Token: "github_pat_secret"}
	if err := a.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(a.Path())
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("Save() wrote mode %04o, want 0600 (the file holds a token)", perm)
	}
}

// A config written by an older guppy is already on disk at 0644. Re-saving it
// has to tighten it, which os.WriteFile alone does not do.
func TestSaveTightensPermissionsOnExistingFile(t *testing.T) {
	testRoot(t)

	dir, err := AppsDir()
	if err != nil {
		t.Fatalf("AppsDir() error: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	path := filepath.Join(dir, "legacy.yaml")
	if err := os.WriteFile(path, []byte("repository:\n  type: github\n  owner: o\n  repo: r\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	a, err := LoadApp("legacy")
	if err != nil {
		t.Fatalf("LoadApp() error: %v", err)
	}
	if err := a.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("Save() left mode %04o on a pre-existing file, want 0600", perm)
	}
}

func TestListApps(t *testing.T) {
	testRoot(t)

	if names, err := ListApps(); err != nil || len(names) != 0 {
		t.Errorf("ListApps() on missing dir = %v, %v; want empty, nil", names, err)
	}

	writeApp(t, "zebra.yaml", githubYAML)
	writeApp(t, "alpha.json", githubJSON)
	writeApp(t, "middle.yml", githubYAML)
	writeApp(t, "notes.txt", "ignored")

	names, err := ListApps()
	if err != nil {
		t.Fatalf("ListApps() error: %v", err)
	}

	want := []string{"alpha", "middle", "zebra"}
	if len(names) != len(want) {
		t.Fatalf("ListApps() = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("ListApps()[%d] = %q, want %q (sorted)", i, names[i], want[i])
		}
	}
}

func TestAppPathRejectsDuplicateNames(t *testing.T) {
	testRoot(t)
	writeApp(t, "dupe.yaml", githubYAML)
	writeApp(t, "dupe.json", githubJSON)

	if _, err := AppPath("dupe"); err == nil {
		t.Error("AppPath() accepted two files claiming the same name, want error")
	}
}

func TestAppPathMissing(t *testing.T) {
	testRoot(t)
	if _, err := AppPath("ghost"); err == nil {
		t.Error("AppPath() on a missing app should error")
	}
}

func TestRemoveApp(t *testing.T) {
	testRoot(t)
	path := writeApp(t, "gone.yaml", githubYAML)

	if err := RemoveApp("gone"); err != nil {
		t.Fatalf("RemoveApp() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("RemoveApp() left the config file behind")
	}
	if err := RemoveApp("gone"); err == nil {
		t.Error("RemoveApp() on a missing app should error")
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "ripgrep", false},
		{"with dash", "my-app", false},
		{"with underscore", "my_app", false},
		{"with dot", "app.v2", false},
		{"with digits", "s3cmd2", false},
		{"empty", "", true},
		{"parent traversal", "../evil", true},
		{"embedded traversal", "a/../../b", true},
		{"absolute path", "/etc/passwd", true},
		{"slash", "owner/repo", true},
		{"backslash", `..\evil`, true},
		{"leading dot", ".hidden", true},
		{"leading dash", "-flag", true},
		{"space", "my app", true},
		{"null byte", "app\x00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateName(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		app     App
		wantErr bool
	}{
		{
			name: "valid github",
			app:  App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r"}, Applier: "binary"},
		},
		{
			name:    "github without owner",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "github", Repo: "r"}, Applier: "binary"},
			wantErr: true,
		},
		{
			name:    "github without repo",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o"}, Applier: "binary"},
			wantErr: true,
		},
		{
			name:    "the http provider is gone",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "http", URL: "https://example.com/r.json"}, Applier: "archive"},
			wantErr: true,
		},
		{
			name:    "repository.url is rejected",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r", URL: "https://example.com/r.json"}, Applier: "binary"},
			wantErr: true,
		},
		{
			name:    "unknown repository type",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "ftp"}, Applier: "binary"},
			wantErr: true,
		},
		{
			name:    "unknown applier",
			app:     App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r"}, Applier: "magic"},
			wantErr: true,
		},
		{
			name: "archive applier may list several binaries",
			app: App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r"},
				Applier: "archive", Bin: []string{"one", "two"}},
		},
		{
			name: "binary applier may not list several binaries",
			app: App{name: "a", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r"},
				Applier: "binary", Bin: []string{"one", "two"}},
			wantErr: true,
		},
		{
			name:    "invalid name",
			app:     App{name: "../evil", Repository: RepositoryConfig{Type: "github", Owner: "o", Repo: "r"}, Applier: "binary"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.app.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// The example configs ship as documentation; if the schema moves they should
// fail here rather than in a user's terminal.
func TestShippedExamplesLoad(t *testing.T) {
	testRoot(t)

	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.yaml"))
	if err != nil {
		t.Fatalf("Glob() error: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("no example configs found")
	}

	appsDir, err := AppsDir()
	if err != nil {
		t.Fatalf("AppsDir() error: %v", err)
	}
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatalf("failed to create apps dir: %v", err)
	}

	for _, example := range examples {
		t.Run(filepath.Base(example), func(t *testing.T) {
			body, err := os.ReadFile(example)
			if err != nil {
				t.Fatalf("failed to read %s: %v", example, err)
			}

			name := strings.TrimSuffix(filepath.Base(example), ".yaml")
			path := writeApp(t, name+".yaml", string(body))

			if _, err := LoadFile(name, path); err != nil {
				t.Errorf("example %s does not load: %v", filepath.Base(example), err)
			}
		})
	}
}
