package common

import (
	"database/sql"
	"testing"
	"time"
)

func TestParseNullTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    sql.NullTime
		expected time.Time
	}{
		{
			name:     "valid time",
			input:    sql.NullTime{Time: now, Valid: true},
			expected: now,
		},
		{
			name:     "null time",
			input:    sql.NullTime{Valid: false},
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseNullTime(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNullInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected sql.NullInt64
	}{
		{
			name:     "positive value",
			input:    42,
			expected: sql.NullInt64{Int64: 42, Valid: true},
		},
		{
			name:     "zero value",
			input:    0,
			expected: sql.NullInt64{Int64: 0, Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullInt64(tt.input)
			if result.Int64 != tt.expected.Int64 || result.Valid != tt.expected.Valid {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestNullFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected sql.NullFloat64
	}{
		{
			name:     "positive value",
			input:    3.14,
			expected: sql.NullFloat64{Float64: 3.14, Valid: true},
		},
		{
			name:     "zero value",
			input:    0.0,
			expected: sql.NullFloat64{Float64: 0.0, Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullFloat64(tt.input)
			if result.Float64 != tt.expected.Float64 || result.Valid != tt.expected.Valid {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected sql.NullString
	}{
		{
			name:     "non-empty string",
			input:    "hello",
			expected: sql.NullString{String: "hello", Valid: true},
		},
		{
			name:     "empty string",
			input:    "",
			expected: sql.NullString{String: "", Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullString(tt.input)
			if result.String != tt.expected.String || result.Valid != tt.expected.Valid {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}
