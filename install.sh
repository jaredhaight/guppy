#!/bin/sh
# Install guppy, and leave guppy managing itself.
#
#   curl -fsSL https://github.com/jaredhaight/guppy/releases/latest/download/install.sh | sh
#
# This script only bootstraps: it downloads a throwaway guppy to a temp
# directory, checks it against the release checksums, and then hands off to
# `guppy add jaredhaight/guppy` followed by `guppy install guppy`. Guppy
# installs itself through its own pipeline, so afterwards it is an ordinary
# managed app and `guppy update guppy` keeps it current. That costs one extra
# download and saves this script from having to know guppy's config format.
set -eu

REPO="jaredhaight/guppy"
BASE="https://github.com/$REPO/releases/latest/download"

fail() {
	echo "install.sh: $*" >&2
	exit 1
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
linux | darwin) ;;
mingw* | msys* | cygwin* | windows*)
	fail "this script is for Linux and macOS; on Windows run install.ps1:
  irm https://github.com/$REPO/releases/latest/download/install.ps1 | iex"
	;;
*) fail "unsupported operating system: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch="amd64" ;;
aarch64 | arm64) arch="arm64" ;;
*) fail "unsupported architecture: $arch (guppy publishes amd64 and arm64)" ;;
esac

asset="guppy-$os-$arch"

command -v curl >/dev/null 2>&1 || fail "curl is required"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset..."
curl -fsSL "$BASE/$asset" -o "$tmp/guppy" || fail "could not download $BASE/$asset"
curl -fsSL "$BASE/checksums.txt" -o "$tmp/checksums.txt" || fail "could not download $BASE/checksums.txt"

# checksums.txt proves only that the file matches what the release published.
# The signed provenance attestation is the stronger check:
#   gh attestation verify <binary> --repo jaredhaight/guppy
expected=$(awk -v want="$asset" '$2 == want { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "checksums.txt has no entry for $asset"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/guppy" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/guppy" | awk '{print $1}')
else
	fail "need sha256sum or shasum to verify the download"
fi

[ "$expected" = "$actual" ] || fail "checksum mismatch for $asset
  expected $expected
  got      $actual"

echo "✓ checksum verified"
chmod +x "$tmp/guppy"

# Re-running the script should update rather than fail: `guppy add` refuses to
# overwrite an app that already has a config.
#
# Read the names out of the table only, starting after the header row. With no
# apps configured guppy prints prose instead of a table, and that prose
# suggests `guppy add ...` — close enough to a table row to match if you just
# skip the first line.
if "$tmp/guppy" list 2>/dev/null |
	awk '$1 == "APP" { table = 1; next } table { print $1 }' |
	grep -qx guppy; then
	"$tmp/guppy" update guppy
else
	"$tmp/guppy" add "$REPO" --asset "$asset"
	"$tmp/guppy" install guppy
fi

bin=$("$tmp/guppy" bin)
case ":$PATH:" in
*":$bin:"*)
	echo
	echo "Run 'guppy update' to keep everything current, guppy included."
	;;
*)
	echo
	echo "Add guppy to your PATH:"
	echo "  export PATH=\"$bin:\$PATH\""
	echo
	echo "Then run 'guppy update' to keep everything current, guppy included."
	;;
esac
