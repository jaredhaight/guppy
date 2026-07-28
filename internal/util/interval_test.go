package util

import (
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "15 minutes",
			input:    "15m",
			expected: 15 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "1 hour",
			input:    "1h",
			expected: 1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "exactly the minimum",
			input:    "5m",
			expected: MinInterval,
			wantErr:  false,
		},
		{
			name:     "1 day",
			input:    "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "2 days",
			input:    "2d",
			expected: 48 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "HH:MM:SS format - 1 hour 30 minutes",
			input:    "01:30:00",
			expected: 1*time.Hour + 30*time.Minute,
			wantErr:  false,
		},
		{
			name:     "HH:MM:SS format - 15 minutes",
			input:    "00:15:00",
			expected: 15 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "MM:SS format - 5 minutes 30 seconds",
			input:    "05:30",
			expected: 5*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			name:     "Combined duration - 1 day 12 hours",
			input:    "1d12h",
			expected: 36 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "Combined duration - 1 hour 30 minutes 45 seconds",
			input:    "1h30m45s",
			expected: 1*time.Hour + 30*time.Minute + 45*time.Second,
			wantErr:  false,
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "Negative value",
			input:   "-1h",
			wantErr: true,
		},
		{
			name:    "Zero duration",
			input:   "0s",
			wantErr: true,
		},
		// Every tick costs an API call per app, so a too-short interval gets
		// the user rate-limited rather than updated.
		{
			name:    "below the minimum",
			input:   "30s",
			wantErr: true,
		},
		{
			name:    "just below the minimum",
			input:   "4m59s",
			wantErr: true,
		},
		{
			name:    "below the minimum in HH:MM:SS form",
			input:   "00:01:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseInterval(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseInterval(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseInterval(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseInterval(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
