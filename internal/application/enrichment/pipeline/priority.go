package pipeline

import (
	"time"
)

// Priority tiers for enrichment queue ordering.
// Higher priority items are processed first.
const (
	// PriorityInteractive is for items the user is actively viewing.
	// These should be enriched immediately.
	PriorityInteractive = 1000

	// PriorityToday is for content released/added today.
	PriorityToday = 200

	// PriorityThisWeek is for content released/added within the last 7 days.
	PriorityThisWeek = 150

	// PriorityThisMonth is for content released/added within the last 30 days.
	PriorityThisMonth = 100

	// PriorityThisYear is for content released/added within the last year.
	PriorityThisYear = 50

	// PriorityOlder is for older content.
	PriorityOlder = 0
)

// CalculatePriority determines the enrichment priority based on estimated release date.
// Newer content gets higher priority to surface fresh additions faster.
func CalculatePriority(releaseDate time.Time) int {
	if releaseDate.IsZero() {
		return PriorityOlder
	}

	age := time.Since(releaseDate)

	switch {
	case age < 24*time.Hour:
		return PriorityToday
	case age < 7*24*time.Hour:
		return PriorityThisWeek
	case age < 30*24*time.Hour:
		return PriorityThisMonth
	case age < 365*24*time.Hour:
		return PriorityThisYear
	default:
		return PriorityOlder
	}
}

// EstimateReleaseDate estimates the release date from available metadata.
// Priority order:
//  1. Parsed year from filename (assume July 1 for middle of year)
//  2. File modification time (fallback)
//
// Note: Actual release date from NFO/TMDB is applied later via re-prioritization.
func EstimateReleaseDate(year *int, fileMTime time.Time) time.Time {
	// If we have a parsed year, use July 1 of that year as an estimate
	// (middle of year is a reasonable guess for theatrical releases)
	if year != nil && *year > 1800 && *year < 2200 {
		return time.Date(*year, time.July, 1, 0, 0, 0, 0, time.UTC)
	}

	// Fall back to file modification time
	if !fileMTime.IsZero() {
		return fileMTime
	}

	// No information available - use zero time (will get PriorityOlder)
	return time.Time{}
}

// CalculatePriorityFromMetadata is a convenience function that combines
// EstimateReleaseDate and CalculatePriority.
func CalculatePriorityFromMetadata(year *int, fileMTime time.Time) int {
	releaseDate := EstimateReleaseDate(year, fileMTime)
	return CalculatePriority(releaseDate)
}
