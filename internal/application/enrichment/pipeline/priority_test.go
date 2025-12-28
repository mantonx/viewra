package pipeline

import (
	"testing"
	"time"
)

func TestCalculatePriority(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		releaseDate time.Time
		expected    int
	}{
		{
			name:        "zero time returns older priority",
			releaseDate: time.Time{},
			expected:    PriorityOlder,
		},
		{
			name:        "released 1 hour ago returns today priority",
			releaseDate: now.Add(-1 * time.Hour),
			expected:    PriorityToday,
		},
		{
			name:        "released 23 hours ago returns today priority",
			releaseDate: now.Add(-23 * time.Hour),
			expected:    PriorityToday,
		},
		{
			name:        "released 2 days ago returns this week priority",
			releaseDate: now.Add(-2 * 24 * time.Hour),
			expected:    PriorityThisWeek,
		},
		{
			name:        "released 6 days ago returns this week priority",
			releaseDate: now.Add(-6 * 24 * time.Hour),
			expected:    PriorityThisWeek,
		},
		{
			name:        "released 10 days ago returns this month priority",
			releaseDate: now.Add(-10 * 24 * time.Hour),
			expected:    PriorityThisMonth,
		},
		{
			name:        "released 29 days ago returns this month priority",
			releaseDate: now.Add(-29 * 24 * time.Hour),
			expected:    PriorityThisMonth,
		},
		{
			name:        "released 60 days ago returns this year priority",
			releaseDate: now.Add(-60 * 24 * time.Hour),
			expected:    PriorityThisYear,
		},
		{
			name:        "released 364 days ago returns this year priority",
			releaseDate: now.Add(-364 * 24 * time.Hour),
			expected:    PriorityThisYear,
		},
		{
			name:        "released 2 years ago returns older priority",
			releaseDate: now.Add(-2 * 365 * 24 * time.Hour),
			expected:    PriorityOlder,
		},
		{
			name:        "released 10 years ago returns older priority",
			releaseDate: now.Add(-10 * 365 * 24 * time.Hour),
			expected:    PriorityOlder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatePriority(tt.releaseDate)
			if got != tt.expected {
				t.Errorf("CalculatePriority() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestEstimateReleaseDate(t *testing.T) {
	now := time.Now()
	year2024 := 2024
	year1999 := 1999
	invalidYear := 1500

	tests := []struct {
		name      string
		year      *int
		fileMTime time.Time
		checkFunc func(t *testing.T, result time.Time)
	}{
		{
			name:      "uses year when provided",
			year:      &year2024,
			fileMTime: now,
			checkFunc: func(t *testing.T, result time.Time) {
				if result.Year() != 2024 {
					t.Errorf("expected year 2024, got %d", result.Year())
				}
				if result.Month() != time.July {
					t.Errorf("expected July, got %s", result.Month())
				}
				if result.Day() != 1 {
					t.Errorf("expected day 1, got %d", result.Day())
				}
			},
		},
		{
			name:      "uses year 1999",
			year:      &year1999,
			fileMTime: now,
			checkFunc: func(t *testing.T, result time.Time) {
				if result.Year() != 1999 {
					t.Errorf("expected year 1999, got %d", result.Year())
				}
			},
		},
		{
			name:      "falls back to mtime when year is nil",
			year:      nil,
			fileMTime: now,
			checkFunc: func(t *testing.T, result time.Time) {
				if !result.Equal(now) {
					t.Errorf("expected mtime %v, got %v", now, result)
				}
			},
		},
		{
			name:      "falls back to mtime when year is invalid",
			year:      &invalidYear,
			fileMTime: now,
			checkFunc: func(t *testing.T, result time.Time) {
				if !result.Equal(now) {
					t.Errorf("expected mtime %v, got %v", now, result)
				}
			},
		},
		{
			name:      "returns zero time when no data available",
			year:      nil,
			fileMTime: time.Time{},
			checkFunc: func(t *testing.T, result time.Time) {
				if !result.IsZero() {
					t.Errorf("expected zero time, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateReleaseDate(tt.year, tt.fileMTime)
			tt.checkFunc(t, result)
		})
	}
}

func TestCalculatePriorityFromMetadata(t *testing.T) {
	now := time.Now()
	currentYear := now.Year()
	oldYear := 1999

	tests := []struct {
		name      string
		year      *int
		fileMTime time.Time
		expected  int
	}{
		{
			name:      "current year with current mtime gets this year priority",
			year:      &currentYear,
			fileMTime: now,
			expected:  PriorityThisYear, // July 1 of current year is within 365 days
		},
		{
			name:      "old year gets older priority",
			year:      &oldYear,
			fileMTime: now,
			expected:  PriorityOlder,
		},
		{
			name:      "no year but recent mtime gets appropriate priority",
			year:      nil,
			fileMTime: now.Add(-2 * 24 * time.Hour),
			expected:  PriorityThisWeek,
		},
		{
			name:      "no data gets older priority",
			year:      nil,
			fileMTime: time.Time{},
			expected:  PriorityOlder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatePriorityFromMetadata(tt.year, tt.fileMTime)
			if got != tt.expected {
				t.Errorf("CalculatePriorityFromMetadata() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestPriorityConstants(t *testing.T) {
	// Verify priority ordering: Interactive > Today > Week > Month > Year > Older
	if PriorityInteractive <= PriorityToday {
		t.Error("Interactive priority should be higher than Today")
	}
	if PriorityToday <= PriorityThisWeek {
		t.Error("Today priority should be higher than ThisWeek")
	}
	if PriorityThisWeek <= PriorityThisMonth {
		t.Error("ThisWeek priority should be higher than ThisMonth")
	}
	if PriorityThisMonth <= PriorityThisYear {
		t.Error("ThisMonth priority should be higher than ThisYear")
	}
	if PriorityThisYear <= PriorityOlder {
		t.Error("ThisYear priority should be higher than Older")
	}
}
