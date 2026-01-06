package home

import "testing"

func TestFormatRemainingTime(t *testing.T) {
	tests := []struct {
		name            string
		positionSeconds int
		durationSeconds int
		expected        string
	}{
		{
			name:            "1 hour 23 minutes remaining",
			positionSeconds: 1800, // 30 min watched
			durationSeconds: 6780, // 1h 53m total
			expected:        "1h 23m left",
		},
		{
			name:            "45 minutes remaining",
			positionSeconds: 900,  // 15 min watched
			durationSeconds: 3600, // 1 hour total
			expected:        "45 min left",
		},
		{
			name:            "2 hours remaining",
			positionSeconds: 0,
			durationSeconds: 7200,
			expected:        "2h left",
		},
		{
			name:            "1 minute remaining minimum",
			positionSeconds: 3590,
			durationSeconds: 3600,
			expected:        "1 min left",
		},
		{
			name:            "no time remaining",
			positionSeconds: 3600,
			durationSeconds: 3600,
			expected:        "",
		},
		{
			name:            "over duration",
			positionSeconds: 4000,
			durationSeconds: 3600,
			expected:        "",
		},
		{
			name:            "zero duration",
			positionSeconds: 100,
			durationSeconds: 0,
			expected:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatRemainingTime(tt.positionSeconds, tt.durationSeconds)
			if result != tt.expected {
				t.Errorf("FormatRemainingTime(%d, %d) = %q, want %q",
					tt.positionSeconds, tt.durationSeconds, result, tt.expected)
			}
		})
	}
}

func TestCalculateProgressPercent(t *testing.T) {
	tests := []struct {
		name            string
		positionSeconds int
		durationSeconds int
		expected        int
	}{
		{
			name:            "50% progress",
			positionSeconds: 1800,
			durationSeconds: 3600,
			expected:        50,
		},
		{
			name:            "0% progress",
			positionSeconds: 0,
			durationSeconds: 3600,
			expected:        0,
		},
		{
			name:            "100% progress",
			positionSeconds: 3600,
			durationSeconds: 3600,
			expected:        100,
		},
		{
			name:            "over 100% clamped",
			positionSeconds: 4000,
			durationSeconds: 3600,
			expected:        100,
		},
		{
			name:            "zero duration",
			positionSeconds: 100,
			durationSeconds: 0,
			expected:        0,
		},
		{
			name:            "75% progress",
			positionSeconds: 2700,
			durationSeconds: 3600,
			expected:        75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateProgressPercent(tt.positionSeconds, tt.durationSeconds)
			if result != tt.expected {
				t.Errorf("CalculateProgressPercent(%d, %d) = %d, want %d",
					tt.positionSeconds, tt.durationSeconds, result, tt.expected)
			}
		})
	}
}
