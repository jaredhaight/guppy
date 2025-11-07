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
- `type` (required): Repository type. Currently supports `github`
- `owner` (required): Repository owner/organization name
- `repo` (required): Repository name
- `token` (optional): GitHub personal access token for private repos or higher rate limits
- `asset_name` (optional): Specific asset name to download. If not specified, uses the first asset

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
- If not specified, guppy automatically uses the appropriate temp directory for your operating system

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

Create a template configuration file in the current directory.

```bash
guppy init
```

This creates a `guppy.json` file with default values that you can customize for your application.

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

## Checksum Verification

Guppy automatically verifies SHA256 checksums if they are provided in the GitHub release. This ensures the downloaded file hasn't been corrupted or tampered with.

If checksum verification fails, the update will not be applied.

## Supported Archive Formats

The archive applier supports:
- `.zip` files
- `.tar.gz` and `.tgz` files

## Security Considerations

1. **GitHub Token**: Store your GitHub token securely. Consider using environment variables or secure configuration management.
2. **Target Path Permissions**: Ensure guppy has appropriate permissions to write to the target path.
3. **Archive Extraction**: The archive applier includes protection against path traversal attacks (ZipSlip).

## Troubleshooting

### Error: "repository owner is required"
Make sure your config file includes all required fields in the repository section.

### Error: "checksum verification failed"
The downloaded file may be corrupted. Try downloading again or check if the release is valid.

### Error: "error applying update: permission denied"
Guppy needs write permissions to the target path. Try running with appropriate permissions or use `sudo` if necessary.

## Integration

Guppy can be integrated into automation workflows:

```bash
# Check for updates and only update if available
if guppy check | grep -q "New version available"; then
    guppy update
    systemctl restart myapp
fi
```

## Development

To contribute to Guppy or build from source:

```bash
git clone https://github.com/jaredhaight/guppy
cd guppy
go build -o guppy ./cmd/guppy
go test ./...
```
