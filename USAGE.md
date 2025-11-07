# Guppy Usage Guide

Guppy is a software update helper that checks for new releases, downloads them, and applies the new version.

## Installation
You can download the latest version of guppy from the [release page](https://github.com/jaredhaight/guppy/releases) or you can build from source:

```bash
git clone https://github.com/jaredhaight/guppy
cd guppy
go build -o guppy ./cmd/guppy
```

## Configuration

Guppy uses a JSON configuration file. By default, it looks for `guppy.json` in the same directory as the guppy executable. You can specify a custom config file location using the `--config` flag (see Command-Line Flags below).

### Configuration File Format

```json
{
  "repository": {
    "type": "github",
    "owner": "username",
    "repo": "project",
    "token": "",
    "asset_name": "project-linux-amd64"
  },
  "current_version": "1.0.0",
  "target_path": "/usr/local/bin/myapp",
  "applier": "binary"
}
```

### Configuration Fields

#### repository
- `type` (required): Repository type. Supports `github` and `http`

**For GitHub repositories:**
- `owner` (required): Repository owner/organization name
- `repo` (required): Repository name
- `token` (optional): GitHub personal access token for private repos or higher rate limits
- `asset_name` (optional): Specific asset name to download. If not specified, uses the first asset

**For HTTP repositories:**
- `url` (required): URL to the releases.json file containing release information

#### current_version
- Current version of the software using Sematic versioning (e.g., "v1.0.0" or "2025.1107.01", etc). 
  - This value is updated (or set) once a new version has been downloaded. 

#### target_path
- Path where the update should be applied
  - For binary applier: path to the binary file
  - For archive applier: directory where archive will be extracted

#### applier
- Type of applier to use. Options:
  - `binary`: Replace a single binary file
  - `archive`: Extract a zip or tar.gz archive

#### download_dir (optional)
- Directory where releases are downloaded
- Default: `{OS_TEMP_DIR}/guppy` (e.g., `/tmp/guppy` on Linux/macOS, `C:\Users\{USERNAME}\AppData\Local\Temp\guppy` on Windows)

## Command-Line Flags

Guppy supports the following command-line flags:

### --config, -c
Specify a custom configuration file path.

```bash
guppy check --config /path/to/config.json
guppy update -c /path/to/config.json
```

### --debug, -d
Enable debug logging for troubleshooting.

```bash
guppy check --debug
guppy update -d
```

## Commands

### guppy check

Check for available updates without downloading or applying them.

```bash
guppy check
```

Example output:
```
Checking for updates...
Latest version: v2.0.0
Current version: v1.0.0
🎉 New version available: v2.0.0
Download URL: https://github.com/user/project/releases/download/v2.0.0/project-linux-amd64
```

### guppy update

Download and apply available updates.

```bash
guppy update
```

Example output:
```
Checking for updates...
Downloading version v2.0.0...
Downloaded to: /tmp/guppy/project-linux-amd64
Verifying checksum...
✓ Checksum verified
Applying update to /usr/local/bin/myapp...
✓ Update applied successfully!
```

### guppy version

Show the version of guppy itself.

```bash
guppy version
```

### guppy init

Create a template configuration file for a specific repository type.

**Usage:**
```bash
guppy init [type]
```

**Arguments:**
- `type` (optional): Repository type - either `github` or `http`
  - If not specified, guppy will prompt you to select a type interactively

**Examples:**

Create a GitHub repository template:
```bash
guppy init github
```

Create an HTTP repository template:
```bash
guppy init http
```

Interactive mode (prompts for repository type):
```bash
guppy init
```

This creates a `guppy.json` file with default values that you can customize for your application. The template will be tailored to the selected repository type:

- **GitHub template**: Includes fields for `owner`, `repo`, `token`, and `asset_name`
- **HTTP template**: Includes field for `url` pointing to your releases.json file

## Examples

### Example 1: Binary Update

Update a single binary application from GitHub releases.

**Config file (guppy.json):**
```json
{
  "repository": {
    "type": "github",
    "owner": "cli",
    "repo": "cli",
    "asset_name": "gh_linux_amd64.tar.gz"
  },
  "current_version": "2.0.0",
  "target_path": "/usr/local/bin/gh",
  "applier": "binary",
  "download_dir": "/tmp/guppy"
}
```

**Usage:**
```bash
guppy check    # Check for updates
guppy update   # Apply updates
```

### Example 2: Archive Extraction

Extract an archive containing multiple files.

**Config file:**
```json
{
  "repository": {
    "type": "github",
    "owner": "user",
    "repo": "app",
    "asset_name": "app-linux.tar.gz"
  },
  "current_version": "1.0.0",
  "target_path": "/opt/myapp",
  "applier": "archive",
  "download_dir": "/tmp/guppy"
}
```

### Example 3: Private Repository

Use a GitHub token for private repositories.

**Config file:**
```json
{
  "repository": {
    "type": "github",
    "owner": "myorg",
    "repo": "private-app",
    "token": "ghp_xxxxxxxxxxxxxxxxxxxx",
    "asset_name": "app-linux-amd64"
  },
  "current_version": "1.0.0",
  "target_path": "/usr/local/bin/private-app",
  "applier": "binary"
}
```

### Example 4: HTTP Repository

Update from a custom web server using the HTTP repository type.

**Config file:**
```json
{
  "repository": {
    "type": "http",
    "url": "https://updates.example.com/myapp/releases.json"
  },
  "current_version": "1.0.0",
  "target_path": "/usr/local/bin/myapp",
  "applier": "binary"
}
```

**releases.json format:**
```json
[
  {
    "version": "2025.281.3",
    "url": "https://updates.example.com/myapp/download-v3.zip",
    "sha256": "997c3ad2cd376d4cc609c3879b831fcfcf785cea14b427c8d7bfc40f77e0c3eb"
  },
  {
    "version": "2025.281.2",
    "url": "https://updates.example.com/myapp/download-v2.zip",
    "sha1": "367c432837f71657db863dae11a71202414f36d8"
  },
  {
    "version": "2025.281.1",
    "url": "https://updates.example.com/myapp/download-v1.zip",
    "md5": "d1c47df9c7d692538e6744fea9d826b1"
  }
]
```

**Notes:**
- The `releases.json` file must be a JSON array of release objects
- Each release must have a `version` and `url` field
- Checksums are optional but recommended. Supported algorithms: `sha256`, `sha1`, `md5`
- If multiple checksums are provided, guppy uses the highest security algorithm (SHA256 > SHA1 > MD5)

## Checksum Verification

Guppy automatically verifies checksums to ensure the downloaded file hasn't been corrupted or tampered with.

**For GitHub repositories:**
- Guppy uses SHA256 checksums if provided in the GitHub release asset digest

**For HTTP repositories:**
- You can specify `sha256`, `sha1`, or `md5` checksums in the releases.json file
- If multiple checksums are provided, guppy uses the most secure algorithm available (SHA256 > SHA1 > MD5)

If checksum verification fails, the downloaded file will be deleted and the update will not be applied.

## Supported Archive Formats

The archive applier supports:
- `.zip` files
- `.tar.gz` and `.tgz` files
