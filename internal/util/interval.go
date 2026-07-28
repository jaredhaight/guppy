package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MinInterval is the shortest polling interval guppy accepts.
//
// Every tick is at least one API call per configured app. GitHub allows 60
// unauthenticated requests an hour, so "-i 30s" would exhaust the budget in
// minutes and get the user rate-limited rather than updated. Nothing releases
// software often enough for a shorter interval to find anything.
const MinInterval = 5 * time.Minute

// ParseInterval parses interval strings in various formats and returns a time.Duration.
// Supported formats:
// - Duration format: "1d", "24h", "15m", "30s" (supports combinations like "1h30m")
// - HH:MM:SS format: "01:30:00", "00:15:00"
func ParseInterval(interval string) (time.Duration, error) {
	d, err := parseInterval(interval)
	if err != nil {
		return 0, err
	}
	if d < MinInterval {
		return 0, fmt.Errorf("interval %s is shorter than the %s minimum", d, MinInterval)
	}
	return d, nil
}

func parseInterval(interval string) (time.Duration, error) {
	if interval == "" {
		return 0, fmt.Errorf("interval cannot be empty")
	}

	// Try to parse as HH:MM:SS format first
	if duration, err := parseHHMMSS(interval); err == nil {
		return duration, nil
	}

	// Try to parse as duration format with day support
	if duration, err := parseDurationWithDays(interval); err == nil {
		return duration, nil
	}

	return 0, fmt.Errorf("invalid interval format: %s (expected formats: 1d, 15m, 30s, or HH:MM:SS)", interval)
}

// parseHHMMSS parses time in HH:MM:SS format
func parseHHMMSS(s string) (time.Duration, error) {
	// Match HH:MM:SS or MM:SS format
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid HH:MM:SS format")
	}

	var hours, minutes, seconds int
	var err error

	if len(parts) == 3 {
		// HH:MM:SS format
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours: %w", err)
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		seconds, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %w", err)
		}
	} else {
		// MM:SS format
		minutes, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		seconds, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %w", err)
		}
	}

	// Validate ranges
	if hours < 0 || minutes < 0 || minutes >= 60 || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("time values out of range")
	}

	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second

	if duration <= 0 {
		return 0, fmt.Errorf("interval must be greater than 0")
	}

	return duration, nil
}

// parseDurationWithDays parses duration strings with support for days (e.g., "1d", "2d12h")
func parseDurationWithDays(s string) (time.Duration, error) {
	// Replace 'd' with 'h' after converting days to hours
	// Use regex to find all instances of numbers followed by 'd'
	re := regexp.MustCompile(`(\d+)d`)
	matches := re.FindAllStringSubmatch(s, -1)

	modifiedString := s
	for _, match := range matches {
		if len(match) >= 2 {
			days, err := strconv.Atoi(match[1])
			if err != nil {
				return 0, fmt.Errorf("invalid day value: %w", err)
			}
			hours := days * 24
			// Replace "Xd" with "Xh" where X is days * 24
			modifiedString = strings.Replace(modifiedString, match[0], fmt.Sprintf("%dh", hours), 1)
		}
	}

	// Now parse as standard Go duration
	duration, err := time.ParseDuration(modifiedString)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %w", err)
	}

	if duration <= 0 {
		return 0, fmt.Errorf("interval must be greater than 0")
	}

	return duration, nil
}
