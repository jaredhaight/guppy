# Guppy Examples

Example app configurations. Each file defines one app; the filename is the app's name.

| File | Shows |
|---|---|
| `ripgrep.yaml` | An archive release whose binary sits inside a versioned directory |
| `myservice.yaml` | `pre_install` / `post_install` hooks around a service restart |

## Using them

Most of the time you don't need to copy anything — `guppy add` writes the config for you:

```bash
guppy add BurntSushi/ripgrep --applier archive --bin rg
```

To use a file directly, drop it in guppy's apps folder. That's `~/.config/guppy/apps` on Linux, `~/Library/Application Support/guppy/apps` on macOS, and `%AppData%\guppy\apps` on Windows:

```bash
cp ripgrep.yaml ~/.config/guppy/apps/
```

Then install it:

```bash
guppy install ripgrep
```

Binaries land in the folder `guppy bin` prints. Add it to your PATH:

```bash
export PATH="$(guppy bin):$PATH"
```

JSON works too, if you prefer it — name the file `ripgrep.json` and use the same keys.

See [USAGE.md](../USAGE.md) for the full set of configuration options.
