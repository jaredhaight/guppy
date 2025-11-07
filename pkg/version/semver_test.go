package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input       string
		expected    *Version
		shouldError bool
	}{
		{
			input: "1.2.3",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			shouldError: false,
		},
		{
			input: "v1.2.3",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			shouldError: false,
		},
		{
			input: "1.2.3-beta.1",
			expected: &Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				PreRelease: "beta.1",
			},
			shouldError: false,
		},
		{
			input: "1.2.3+build.123",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
			},
			shouldError: false,
		},
		{
			input:       "invalid",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Parse(tt.input)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Major != tt.expected.Major ||
				result.Minor != tt.expected.Minor ||
				result.Patch != tt.expected.Patch ||
				result.PreRelease != tt.expected.PreRelease ||
				result.Build != tt.expected.Build {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.1.0", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0-beta", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			ver1, err := Parse(tt.v1)
			if err != nil {
				t.Fatalf("error parsing v1: %v", err)
			}

			ver2, err := Parse(tt.v2)
			if err != nil {
				t.Fatalf("error parsing v2: %v", err)
			}

			result := ver1.Compare(ver2)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"2.0.0", "1.0.0", true},
		{"1.0.0", "2.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.1", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			result, err := IsNewer(tt.v1, tt.v2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
