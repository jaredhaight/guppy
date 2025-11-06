package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

// Parse parses a semantic version string
func Parse(v string) (*Version, error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Split build metadata
	parts := strings.Split(v, "+")
	v = parts[0]
	build := ""
	if len(parts) > 1 {
		build = parts[1]
	}

	// Split pre-release
	parts = strings.Split(v, "-")
	v = parts[0]
	preRelease := ""
	if len(parts) > 1 {
		preRelease = strings.Join(parts[1:], "-")
	}

	// Parse major.minor.patch
	parts = strings.Split(v, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Build:      build,
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions
// Returns:
//   -1 if v < other
//    0 if v == other
//    1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major > other.Major {
			return 1
		}
		return -1
	}

	if v.Minor != other.Minor {
		if v.Minor > other.Minor {
			return 1
		}
		return -1
	}

	if v.Patch != other.Patch {
		if v.Patch > other.Patch {
			return 1
		}
		return -1
	}

	// Handle pre-release versions
	// Version without pre-release is greater than with pre-release
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1
	}

	// Both have pre-release, compare lexicographically
	if v.PreRelease != other.PreRelease {
		if v.PreRelease > other.PreRelease {
			return 1
		}
		return -1
	}

	return 0
}

// IsNewer returns true if v is newer than other
func (v *Version) IsNewer(other *Version) bool {
	return v.Compare(other) > 0
}

// IsOlder returns true if v is older than other
func (v *Version) IsOlder(other *Version) bool {
	return v.Compare(other) < 0
}

// Equals returns true if v equals other
func (v *Version) Equals(other *Version) bool {
	return v.Compare(other) == 0
}

// CompareStrings compares two version strings
func CompareStrings(v1, v2 string) (int, error) {
	ver1, err := Parse(v1)
	if err != nil {
		return 0, fmt.Errorf("error parsing version %s: %w", v1, err)
	}

	ver2, err := Parse(v2)
	if err != nil {
		return 0, fmt.Errorf("error parsing version %s: %w", v2, err)
	}

	return ver1.Compare(ver2), nil
}

// IsNewer returns true if v1 is newer than v2
func IsNewer(v1, v2 string) (bool, error) {
	cmp, err := CompareStrings(v1, v2)
	if err != nil {
		return false, err
	}
	return cmp > 0, nil
}
