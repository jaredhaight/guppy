# Guppy Usage Guide

Guppy manages applications published as GitHub releases or served from your own web server. It checks for new versions, downloads and verifies them, installs them, and links their binaries into a folder on your PATH.

## Installation

You can download the latest version of guppy from the [release page](https://github.com/jaredhaight/guppy/releases) or you can build from source:

```bash
git clone https://github.com/jaredhaight/guppy
cd guppy
go build -o guppy ./cmd/guppy
```

### Verifying a downloaded release

Guppy's own releases are built by GitHub Actions and carry a signed provenance attestation, so you can confirm a binary came from this repository rather than trusting the download:

```bash
gh attestation verify guppy-linux-amd64 --repo jaredhaight/guppy
```

Guppy asks the same of the software it installs, so it's worth applying to guppy itself. The `checksums.txt` published alongside the binaries is a convenience, not evidence — it's unsigned, and anyone able to replace a binary can replace it too.

Then add guppy's bin folder to your PATH:

```bash
export PATH="$(guppy bin):$PATH"
```

Put that line in your shell profile (`~/.zshrc`, `~/.bashrc`) to make it permanent.

## Where guppy keeps things

Guppy uses the standard configuration and data locations for your operating system.

| | Config | Data |
|---|---|---|
| Linux | `~/.config/guppy` | `~/.local/share/guppy` |
| macOS | `~/Library/Application Support/guppy` | `~/Library/Application Support/guppy` |
| Windows | `%AppData%\guppy` | `%LocalAppData%\guppy` |

Within those:

```
<config>/apps/<name>.yaml     one file per managed app
<data>/bin/                   the binaries guppy installs  ← put this on your PATH
<data>/apps/<name>/           where each app's files are installed
<data>/downloads/             transient, cleaned up after each install
```

Both locations can be overridden with the `GUPPY_CONFIG_DIR` and `GUPPY_DATA_DIR` environment variables, or the config location with `--config-dir`. This is useful for keeping a separate set of apps, or for testing.

## Commands

### guppy [app...]

Updates apps. With no arguments it updates every app you've added.

```bash
guppy
```

```bash
guppy ripgrep fd
```

If one app fails, the rest still run, and guppy reports how many failed at the end.

### guppy check [app...]

Reports what's available without downloading or installing anything.

```bash
guppy check
```

```
fd: ✓ up to date (10.2.0)
ripgrep: 🎉 14.1.1 available (current 14.0.3)
```

### guppy add

Starts managing an app. For a GitHub repository:

```bash
guppy add BurntSushi/ripgrep --applier archive --bin rg
```

For an HTTP release feed:

```bash
guppy add --url https://example.com/releases.json --name myapp
```

| Flag | Purpose |
|---|---|
| `--name` | App name. Defaults to the repo name |
| `--applier` | `binary` (default) or `archive` |
| `--bin` | A binary to link into the bin folder. Repeatable |
| `--asset` | Which release asset to download, by name or pattern |
| `--token` | GitHub token, for private repos or higher rate limits. Prefer `GH_TOKEN` in the environment |
| `--url` | Release feed URL, for the http provider |
| `--pre-install` | Shell command to run before installing. Repeatable |
| `--post-install` | Shell command to run after installing. Repeatable |

The app name can't collide with a guppy subcommand — use `--name` if the repo is called something like `list`.

### guppy list

```bash
guppy list
```

```
APP                      VERSION        SOURCE
fd                       10.2.0         sharkdp/fd
ripgrep                  14.1.1         BurntSushi/ripgrep
```

### guppy remove

Deletes the app's config, its install directory, and its bin links. Asks for confirmation unless you pass `--yes`.

```bash
guppy remove ripgrep
```

### guppy bin

Prints the folder guppy links binaries into, so you can add it to your PATH.

```bash
export PATH="$(guppy bin):$PATH"
```

### guppy version

Prints guppy's own version.

## Command-Line Flags

### --config-dir

Use a different config folder. Handy for keeping separate sets of apps.

```bash
guppy --config-dir ~/work-tools list
```

### --debug, -d

Enable debug logging, written to stderr.

```bash
guppy --debug ripgrep
```

### --interval, -i

Keep running, checking at a fixed interval instead of exiting after one pass. Accepts `15m`, `1h`, `1d`, or `HH:MM:SS`.

```bash
guppy --interval 6h
```

```bash
guppy --interval 01:30:00
```

Press Ctrl+C to stop — that also aborts a download in progress rather than waiting for it to finish. Guppy stops cleanly on SIGTERM too, so it works under systemd.

The shortest accepted interval is 5 minutes. Each tick costs at least one API call per app, and GitHub allows 60 an hour unauthenticated, so a shorter interval gets you rate-limited rather than updated.

## Configuration

Each app has its own file in `<config>/apps/`. Guppy reads YAML (`.yaml`, `.yml`) and JSON (`.json`); `guppy add` writes YAML. The app's name comes from the filename, so `ripgrep.yaml` defines an app called `ripgrep`.

```yaml
repository:
  type: github
  owner: BurntSushi
  repo: ripgrep
  asset_name: aarch64-apple-darwin\.tar\.gz$
current_version: 14.1.1
applier: archive
bin:
  - rg
pre_install:
  - systemctl --user stop myservice
post_install:
  - rg --version
```

### repository

Where releases come from.

For `type: github`:

| Field | Required | Purpose |
|---|---|---|
| `owner` | yes | Repository owner |
| `repo` | yes | Repository name |
| `token` | no | Personal access token, for private repos or higher rate limits. Overrides `GH_TOKEN`/`GITHUB_TOKEN` from the environment |
| `asset_name` | no | Which asset to download. Defaults to the first one. See below |

For `type: http`:

| Field | Required | Purpose |
|---|---|---|
| `url` | yes | URL of the releases JSON feed |

#### asset_name

Most projects attach one asset per platform, so you need to say which one you want. `asset_name` is matched first as an exact filename, and failing that as a regular expression.

The pattern form is usually what you want, because asset names embed the version:

```
ripgrep-15.2.0-aarch64-apple-darwin.tar.gz
```

Pinning that exact name means guppy stops finding an asset the day 15.3.0 ships. A pattern keeps working:

```yaml
asset_name: aarch64-apple-darwin\.tar\.gz$
```

Anchor the end with `$`. Many projects publish a `.sha256` file next to each asset, so an unanchored `aarch64-apple-darwin\.tar\.gz` matches both and guppy reports the ambiguity rather than guessing. When a pattern matches nothing, the error lists the assets that release actually has.

### current_version

The version currently installed. Guppy writes this itself after each successful install, so you normally leave it alone. When it's empty, guppy installs the latest release.

### applier

How the download is installed.

- `binary` — the download is a single executable. It's installed into the app's directory and linked into the bin folder.
- `archive` — the download is a `.zip`, `.tar.gz` or `.tgz`. It's extracted into the app's directory, then the binaries named in `bin` are linked.

### bin

Which binaries to link into the bin folder. Defaults to the app's own name.

Each entry is looked up first as a path relative to the install directory, and failing that by searching the extracted files for that filename. This second form is what you usually want, because archives commonly unpack into a versioned directory:

```
ripgrep-14.1.1-x86_64-apple-darwin/rg
```

Writing `bin: [rg]` finds that no matter what the version directory is called. If a name turns out to be ambiguous, guppy tells you which files matched so you can give the relative path instead.

Binaries are linked, not copied, and guppy makes them executable — archives don't always record the executable bit.

### allow_unverified

Drops guppy's integrity requirements for this one app: plain-HTTP URLs are accepted, and so are releases that ship no checksum. Guppy warns on every unverified install. Defaults to `false`. See [Verification](#verification).

### pre_install and post_install

Shell commands to run around the install. Each is a list, run in order, stopping at the first failure.

```yaml
pre_install:
  - systemctl --user stop myservice
post_install:
  - ./bin/migrate --up
  - systemctl --user start myservice
```

These run through your system shell (`sh -c`, or `cmd /c` on Windows), so pipes, redirection and `&&` all work.

They run with the app's install directory as the working directory, and with these variables in the environment:

| Variable | Value |
|---|---|
| `GUPPY_APP` | The app's name |
| `GUPPY_VERSION` | The version being installed |
| `GUPPY_PREVIOUS_VERSION` | The version being replaced, empty on a first install |
| `GUPPY_INSTALL_DIR` | Where the app's files are |
| `GUPPY_BIN_DIR` | Where binaries are linked |
| `GUPPY_DOWNLOAD` | The downloaded file |

**Ordering.** `pre_install` runs *after* the download has been fetched and verified, not before. A failed download therefore never leaves a stopped service behind.

If `pre_install` fails, nothing is installed and the recorded version is unchanged. If `post_install` fails, the install has already happened and is recorded — guppy reports the hook failure and says so.

Because these are shell commands from your own config file, treat an app config the same way you'd treat a shell script: only use ones you trust.

## Examples

### Example 1: A binary release from GitHub

```bash
guppy add jaredhaight/guppy --asset guppy-linux-amd64
```

```bash
guppy guppy
```

### Example 2: An archive with a nested binary

```bash
guppy add BurntSushi/ripgrep --applier archive --bin rg --asset 'x86_64-unknown-linux-musl\.tar\.gz$'
```

### Example 3: A private repository

```bash
guppy add myorg/internal-tool --token github_pat_xxxxxxxxxxxx
```

Better still, don't put it anywhere on disk. Guppy reads `GUPPY_GITHUB_TOKEN`, `GH_TOKEN` and `GITHUB_TOKEN` (in that order) when an app config has no `token` of its own, so the same variable the GitHub CLI and CI already set works here:

```bash
export GH_TOKEN=github_pat_xxxxxxxxxxxx
guppy add myorg/internal-tool
```

A `token` in the config file takes precedence, since it's chosen per app. Config files are written `0600` because of it. `--token` still works but leaves the credential in your shell history, so prefer the environment.

### Example 4: A service that needs restarting

`~/.config/guppy/apps/myservice.yaml`

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

### Example 5: An HTTP release feed

```bash
guppy add --url https://example.com/releases.json --name myapp --applier archive --bin myapp
```

The feed is a JSON array:

```json
[
  {
    "version": "1.0.0",
    "url": "https://example.com/myapp-1.0.0.tar.gz",
    "sha256": "997c3ad2cd376d4cc609c3879b831fcfcf785cea14b427c8d7bfc40f77e0c3eb"
  },
  {
    "version": "1.1.0",
    "url": "https://example.com/myapp-1.1.0.tar.gz",
    "sha256": "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567a"
  }
]
```

Guppy picks the highest version, so order doesn't matter.

### Example 6: Continuous monitoring

```bash
guppy --interval 6h
```

As a systemd user service:

```ini
[Unit]
Description=Guppy update monitor

[Service]
ExecStart=/usr/local/bin/guppy --interval 6h
Restart=on-failure

[Install]
WantedBy=default.target
```

## Verification

Guppy installs binaries onto your `PATH`, so it refuses to install anything it can't verify.

- **GitHub** supplies a SHA256 digest for release assets automatically.
- **HTTP** feeds may include `md5`, `sha1` and/or `sha256`. Guppy uses the strongest one available (sha256 > sha1 > md5), and says so when it had to fall back — MD5 and SHA-1 are broken against deliberate collisions, so an attacker who can choose the artifact can also choose one that matches.

Two requirements, both enforced before anything is downloaded:

1. **URLs must be `https`.** This covers both the feed URL and the artifact URL the feed points at — they're set separately, so a secure feed can still hand back an insecure download. `localhost` and loopback addresses are exempt.
2. **The release must carry a checksum.** A download that fails verification is deleted and the install stops.

Both matter together rather than separately: over plain HTTP, anyone who can rewrite the download can rewrite the checksum alongside it, so verification proves nothing.

### Overriding this

If you're installing from a source that can't meet either requirement, set `allow_unverified` on that app:

```yaml
repository:
  type: http
  url: http://internal-host.lan/releases.json
allow_unverified: true
```

Guppy will then install from plain-HTTP URLs and accept releases with no checksum, printing a warning each time it does. It's per app, never global — you're saying you trust that specific source, not switching the protection off.

## Supported Archive Formats

- `.zip`
- `.tar.gz`
- `.tgz`

The format is chosen by the filename. Symlinks and device entries inside archives are skipped, and entries that would write outside the install directory are rejected.

Extraction is staged: files are unpacked alongside the current install and only swapped in once extraction succeeds, so a truncated or corrupt archive leaves your working install intact.

## Exit Status

Guppy exits non-zero if any app fails. When updating several apps, a failure in one doesn't stop the others — guppy reports each failure and finishes with a count.
