# Guppy

Guppy is a package manager for applications published as GitHub releases. It's primarily designed around two use cases.

1. Developers who want a simple update mechanism for their deployed applications. This was my primary idea in making this as the problem comes up with most everything I develop. You can ship guppy along with your app and use it to keep the app up to date.
2. Users who want to keep open source applications hosted on github up to date on their machine. Guppy manages any number of apps, installs their binaries into a folder on your PATH, and keeps them current.

# How it works

Guppy checks GitHub for new releases, from public or private repos. Releases can be a bare binary or an archive (zip, tar.gz). Guppy handles checking for updates, downloading new releases, verifying them, extracting them, and linking their binaries into a `bin` folder you put on your PATH.

For installs that need more than "put the file there" — stopping a service, running a schema migration, clearing a cache — each app can define `pre_install` and `post_install` shell commands.

Releases must be served over `https` and carry a checksum, or guppy refuses to install them. See [Verification](USAGE.md#verification) for the reasoning and the per-app override.

# Getting started

Grab a binary from the [releases page](https://github.com/jaredhaight/guppy/releases), or build from source with `go build ./cmd/guppy`. Guppy's own releases carry a signed provenance attestation — see [Installation](USAGE.md#installation) for how to check it.

Add the bin folder to your PATH:

```bash
export PATH="$(guppy bin):$PATH"
```

Install an app:

```bash
guppy install BurntSushi/ripgrep --applier archive --bin rg
```

Then keep everything current with `guppy update`, which updates every app you've added.

Turn on shell completion while you're here — guppy completes commands, flags, and your app names:

```bash
source <(guppy completion bash)   # or zsh, fish, powershell
```

# Commands

| Command | What it does |
|---|---|
| `guppy install <owner>/<repo>` | Add an app and install it |
| `guppy install <app>` | Install an app you've already added |
| `guppy update [app...]` | Update every app, or just the ones you name |
| `guppy check [app...]` | Report what's available without installing |
| `guppy add <owner>/<repo>` | Start managing an app without installing it |
| `guppy list` | Show managed apps and their versions |
| `guppy remove <app>` | Remove an app and everything guppy installed for it (alias: `rm`) |
| `guppy bin` | Print the folder guppy links binaries into |
| `guppy completion <shell>` | Print a shell completion script |
| `guppy version` | Print guppy's version |

Running `guppy` on its own prints help.

# Configuration

Each app gets its own config file in guppy's config folder. Run `guppy add` to create one, then edit it by hand as needed. YAML and JSON are both accepted — `guppy add` writes YAML.

| | Location |
|---|---|
| Linux | `~/.config/guppy/apps/`, binaries in `~/.local/share/guppy/bin` |
| macOS | `~/Library/Application Support/guppy/apps/`, binaries in `~/Library/Application Support/guppy/bin` |
| Windows | `%AppData%\guppy\apps\`, binaries in `%LocalAppData%\guppy\bin` |

### Example Config

`~/.config/guppy/apps/ripgrep.yaml`

```yaml
repository:
  type: github
  owner: BurntSushi
  repo: ripgrep
  # An exact asset name, or a pattern. Anchored with $ so it does not also
  # match the .tar.gz.sha256 sidecar asset.
  asset_name: aarch64-apple-darwin\.tar\.gz$
  token: github_pat_xxxxxxxxxxxxxxxxxxxx
current_version: 14.1.1
applier: archive
bin:
  - rg
```

### Example with install hooks

```yaml
repository:
  type: github
  owner: myorg
  repo: myservice
applier: archive
bin:
  - myservice
pre_install:
  - systemctl --user stop myservice
post_install:
  - ./bin/migrate --up
  - systemctl --user start myservice
```

Hooks run through your shell, so pipes and `&&` work. They receive `GUPPY_APP`, `GUPPY_VERSION`, `GUPPY_PREVIOUS_VERSION`, `GUPPY_INSTALL_DIR`, `GUPPY_BIN_DIR` and `GUPPY_DOWNLOAD` in their environment, and run with the app's install directory as their working directory.

For more details on how to use Guppy, check [USAGE.md](USAGE.md)
