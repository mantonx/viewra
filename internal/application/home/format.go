package home

import "fmt"

// FormatRemainingTime formats the remaining time as a human-readable string.
// Returns "Xh Ym left" for durations >= 1 hour, "X min left" for shorter durations.
// Returns empty string if remaining time is <= 0.
func FormatRemainingTime(positionSeconds, durationSeconds int) string {
	remaining := durationSeconds - positionSeconds
	if remaining <= 0 {
		return ""
	}

	hours := remaining / 3600
	mins := (remaining % 3600) / 60

	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%dh %dm left", hours, mins)
		}
		return fmt.Sprintf("%dh left", hours)
	}

	if mins <= 0 {
		mins = 1 // Show at least 1 min
	}
	return fmt.Sprintf("%d min left", mins)
}

// CalculateProgressPercent calculates progress as a percentage (0-100).
func CalculateProgressPercent(positionSeconds, durationSeconds int) int {
	if durationSeconds <= 0 {
		return 0
	}
	percent := (positionSeconds * 100) / durationSeconds
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	return percent
}
