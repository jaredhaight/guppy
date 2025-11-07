# Guppy
Guppy is a software update helper. It checks for new releases, downloads them, and updates software. It's designed for two use cases.

1. Developers who want a simple update mechanism for their deployed applications. This was my primary idea in making this as the problem comes up with most everything I develop.
2. Users who want to keep an open source application hosted on github up to date on their machine. Because guppy can be pointed at different config files, you can use it to update your local install of an application straight from the repo.

# How it works
Guppy abstracts providers behind "release repositories" that allow it to check for updates, download them, and verify them. Updates can either be in binary or zip format (zip, tar.gz, etc). Once verified, guppy will then replace either the target binary or the files in the destination folder.

Guppy is designed to be simple, so you may need to wrap it in a script if you need to handle things likes file locks, clearing cached data, seeding databases, etc. 

## Providers
Guppy currently supports two update providers: Github and HTTP. Github leverages github releases and works with public or private repos. HTTP uses a `releases.json` on a web server for release information. The file uses the following format:

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

The md5, sha1, and sha256 hash values are all optional. While you can specify more than one hashing algorithm if you'd like,  Guppy will use the highest value hashing algorithm by default (sha256 > sha1 > md5)

# Configuration
Configuration is handled through a `guppy.json` config file.

## Example Github Config
```json
{
  "applier": "binary",
  "current_version": "v2025.1107.8",
  "download_dir": "/tmp/guppy",
  "repository": {
    "type": "github",
    "owner": "jaredhaight",
    "repo": "guppy",
    "token": "github_pat_xxxxxxxxxxxxxxxxxxxx",
    "asset_name": "guppy-linux-amd64"
  },
  "target_path": "./guppy"
}
```

## Example HTTP Config
```json
{
  "applier": "binary",
  "current_version": "v2025.1107.8",
  "download_dir": "/tmp/guppy",
  "repository": {
    "type": "http",
    "url": "http://www.example.com/releases.json"
  },
  "target_path": "./guppy"
}
```

For more details on how to use Guppy, check [USAGE.md](USAGE.md)