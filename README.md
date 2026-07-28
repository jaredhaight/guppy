# Guppy

Guppy is a package manager for applications published as GitHub releases (or from your own web server). It's primarily designed around two use cases.

1. Developers who want a simple update mechanism for their deployed applications. This was my primary idea in making this as the problem comes up with most everything I develop. You can ship guppy along with your app and use it to keep the app up to date.
2. Users who want to keep open source applications hosted on github up to date on their machine. Guppy manages any number of apps, installs their binaries into a folder on your PATH, and keeps them current.

# How it works

Guppy checks for new releases either through github or your own webserver. Releases can be a bare binary or an archive (zip, tar.gz). Guppy handles checking for updates, downloading new releases, verifying them, extracting them, and linking their binaries into a `bin` folder you put on your PATH.

For installs that need more than "put the file there" — stopping a service, running a schema migration, clearing a cache — each app can define `pre_install` and `post_install` shell commands.

### Providers

Guppy currently supports two update providers: Github and HTTP. The github provider works with github releases from public or private repos. The HTTP provider retrieves a JSON blob of release information from a web server and uses that to determine where to find new releases. The JSON for this is in the following format:

```json
[
    {
        "version": "2025.281.3",
        "url": "https://example.com/download.zip",
        "md5": "d1c47df9c7d692538e6744fea9d826b1",
        "sha1": "367c432837f71657db863dae11a71202414f36d8",
        "sha256": "997c3ad2cd376d4cc609c3879b831fcfcf785cea14b427c8d7bfc40f77e0c3eb"
    }
]
```

This can either be a file stored on a webserver or storage account (http://example.com/release.json) or a regular API endpoint (http://example.com/updates/)

The md5, sha1, and sha256 hash values are all optional. While you can specify more than one hashing algorithm if you'd like, Guppy will use only the most secure hashing algorithm by default (sha256 > sha1 > md5)

# Getting started

Add the bin folder to your PATH:

```bash
export PATH="$(guppy bin):$PATH"
```

Add an app and install it:

```bash
guppy add BurntSushi/ripgrep --applier archive --bin rg
```

```bash
guppy ripgrep
```

Then keep everything current with a bare `guppy`, which updates every app you've added.

# Commands

| Command | What it does |
|---|---|
| `guppy [app...]` | Update every app, or just the ones you name |
| `guppy check [app...]` | Report what's available without installing |
| `guppy add <owner>/<repo>` | Start managing an app |
| `guppy list` | Show managed apps and their versions |
| `guppy remove <app>` | Remove an app and everything guppy installed for it |
| `guppy bin` | Print the folder guppy links binaries into |
| `guppy version` | Print guppy's version |

# Configuration

Each app gets its own config file in guppy's config folder. Run `guppy add` to create one, then edit it by hand as needed. YAML and JSON are both accepted — `guppy add` writes YAML.

| | Location |
|---|---|
| Linux | `~/.config/guppy/apps/`, binaries in `~/.local/share/guppy/bin` |
| macOS | `~/Library/Application Support/guppy/apps/`, binaries in `~/Library/Application Support/guppy/bin` |
| Windows | `%AppData%\guppy\apps\`, binaries in `%LocalAppData%\guppy\bin` |

### Example Github Config

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

### Example HTTP Config

```yaml
repository:
  type: http
  url: http://www.example.com/releases
current_version: v2025.1107.8
applier: binary
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
