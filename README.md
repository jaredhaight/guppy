# Guppy
Guppy is a software update helper.]It's primarly designed around two use cases.

1. Developers who want a simple update mechanism for their deployed applications. This was my primary idea in making this as the problem comes up with most everything I develop. You can ship guppy along with your app and use it to keep the app up to date.
2. Users who want to keep an open source application hosted on github up to date on their machine. Because guppy can be pointed at different config files, you can use it to update your local install of an application straight from the repos releases.

# How it works
Guppy can check for new releases either through github or your own webserver. Releases can be in either binary or zip format (zip, tar.gz, etc). Guppy handles checking for updates, downloading new releases, verifying them, and then copying the contents to a destination.

Guppy is designed to be simple. You will need to wrap it in a script if you need to handle more complext update tasks like stoping services, clearing cached data, schema migrations, etc. 

### Providers
Guppy currently supports two update providers: Github and HTTP. The github provider works with github releases from public or private repos. The HTTP provider retrieves a JSON blob of relase information from a web server and uses that to determine where to find new releases. The JSON for this is in following format:

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

The md5, sha1, and sha256 hash values are all optional. While you can specify more than one hashing algorithm if you'd like,  Guppy will use only the most secure hashing algorithm by default (sha256 > sha1 > md5)

# Configuration
Configuration is handled through a `guppy.json` config file.

### Example Github Config
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

### Example HTTP Config
```json
{
  "applier": "binary",
  "current_version": "v2025.1107.8",
  "download_dir": "/tmp/guppy",
  "repository": {
    "type": "http",
    "url": "http://www.example.com/releases"
  },
  "target_path": "./guppy"
}
```

For more details on how to use Guppy, check [USAGE.md](USAGE.md)