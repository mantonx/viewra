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
// Uses parsed year from filename, assuming July 1 for middle of year estimate.
//
// Note: Actual release date from NFO/TMDB is applied later via re-prioritization.
func EstimateReleaseDate(year *int) time.Time {
	// If we have a parsed year, use July 1 of that year as an estimate
	// (middle of year is a reasonable guess for theatrical releases)
	if year != nil && *year > 1800 && *year < 2200 {
		return time.Date(*year, time.July, 1, 0, 0, 0, 0, time.UTC)
	}

	// No year information available - use zero time (will get PriorityOlder)
	return time.Time{}
}

// CalculatePriorityFromMetadata determines enrichment priority based on two factors:
//  1. Release recency - newer releases (2024-2025) should be enriched before older content
//  2. Addition recency - content added to the library today should be prioritized
//
// The final priority is the maximum of release-based and addition-based priorities.
// This ensures both new releases AND freshly added content get fast enrichment.
//
// Parameters:
//   - year: parsed release year from filename (nil if unknown)
//   - addedAt: when the item was added to the library (typically time.Now() during scan)
func CalculatePriorityFromMetadata(year *int, addedAt time.Time) int {
	// Priority based on release date (new releases should be enriched first)
	releaseDate := EstimateReleaseDate(year)
	releasePriority := CalculatePriority(releaseDate)

	// Priority based on when added to library (fresh additions should be enriched first)
	// This ensures a 1990s movie added today gets priority over a 1990s movie added last month
	addedPriority := CalculatePriority(addedAt)

	// Return the higher priority - content is important if it's EITHER a new release OR newly added
	if releasePriority > addedPriority {
		return releasePriority
	}
	return addedPriority
}
