# Guppy Examples

This directory contains example configuration files for Guppy.

## Basic GitHub Binary Update

The `guppy.json` file shows a basic configuration for updating a binary from GitHub releases.

To use this example:

1. Copy `guppy.json` to your home directory:
   ```bash
   mkdir -p ~/.config/guppy
   cp guppy.json ~/.config/guppy/
   ```

2. Edit the configuration to match your project:
   - Change `owner` and `repo` to your GitHub repository
   - Set `asset_name` to the specific asset you want to download
   - Set `target_path` to where the binary should be installed
   - Optionally add a GitHub token if accessing private repos

3. Run guppy:
   ```bash
   guppy check   # Check for updates
   guppy update  # Download and apply updates
   ```

## Configuration Options

See the [USAGE.md](../USAGE.md) file for complete documentation on all configuration options and examples.
