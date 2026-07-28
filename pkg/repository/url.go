package repository

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

// ValidateURL rejects any URL guppy would fetch over a channel an attacker on
// the network can rewrite.
//
// Plain HTTP is the problem: guppy takes what comes back, verifies it against a
// checksum that arrived over the same channel, and links the result onto the
// user's PATH. An attacker who can rewrite the response can rewrite the
// checksum with it, so the verification proves nothing.
//
// Loopback is exempt, the same carve-out browsers make for secure contexts:
// there is no network segment to sit on, and it is how local test servers and
// self-hosted feeds on the same box are reached.
func ValidateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}

	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("refusing to use insecure URL %q: use https, or set allow_unverified: true to override", raw)
	case "":
		return fmt.Errorf("URL %q has no scheme (want https://...)", raw)
	default:
		return fmt.Errorf("unsupported URL scheme %q in %q (want https)", parsed.Scheme, raw)
	}
}

// ValidateFileName rejects a release-supplied filename that isn't a plain file
// name.
//
// FileName comes from the feed — a GitHub asset's name, or the last segment of
// an artifact URL — and guppy joins it onto the download directory and later
// deletes that path. Anything with a separator, or "..", would let the feed
// pick where guppy writes and what it removes. Checked here, once, where the
// Release is built, rather than at each place the name is used.
//
// Both separators are rejected regardless of platform: filepath.Base doesn't
// split on backslash off Windows, so a `..\..\evil` name would slip through a
// Base-only check on Linux and still be a traversal on Windows.
func ValidateFileName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("release has no file name")
	case name == "." || name == "..":
		return fmt.Errorf("invalid release file name %q", name)
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("invalid release file name %q: must not contain a path separator", name)
	case strings.ContainsRune(name, 0):
		return fmt.Errorf("invalid release file name %q: contains a null byte", name)
	case filepath.Base(name) != name:
		return fmt.Errorf("invalid release file name %q", name)
	}
	return nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
