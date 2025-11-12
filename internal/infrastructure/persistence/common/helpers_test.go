package common

import (
	"database/sql"
	"testing"
	"time"
)

func TestIsPostgres(t *testing.T) {
	tests := []struct {
		driver string
		want   bool
	}{
		{"postgres", true},
		{"postgresql", true},
		{"sqlite", false},
		{"sqlite3", false},
		{"", false},
		{"mysql", false},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			if got := IsPostgres(tt.driver); got != tt.want {
				t.Errorf("IsPostgres(%q) = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

func TestIsSQLite(t *testing.T) {
	tests := []struct {
		driver string
		want   bool
	}{
		{"sqlite", true},
		{"sqlite3", true},
		{"", true}, // Empty defaults to SQLite
		{"postgres", false},
		{"postgresql", false},
		{"mysql", false},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			if got := IsSQLite(tt.driver); got != tt.want {
				t.Errorf("IsSQLite(%q) = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

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

func TestParseTimeInterface(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    interface{}
		expected time.Time
	}{
		{
			name:     "time.Time input",
			input:    now,
			expected: now,
		},
		{
			name:     "sql.NullTime valid",
			input:    sql.NullTime{Time: now, Valid: true},
			expected: now,
		},
		{
			name:     "sql.NullTime invalid",
			input:    sql.NullTime{Valid: false},
			expected: time.Time{},
		},
		{
			name:     "string SQLite format",
			input:    "2024-01-15 10:30:00",
			expected: now,
		},
		{
			name:     "string ISO8601 format",
			input:    "2024-01-15T10:30:00Z",
			expected: now,
		},
		{
			name:     "string RFC3339 format",
			input:    now.Format(time.RFC3339),
			expected: now,
		},
		{
			name:     "invalid string format",
			input:    "not a date",
			expected: time.Time{},
		},
		{
			name:     "unsupported type",
			input:    12345,
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTimeInterface(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    sql.NullString
		expected string
	}{
		{
			name:     "valid string",
			input:    sql.NullString{String: "hello", Valid: true},
			expected: "hello",
		},
		{
			name:     "null string",
			input:    sql.NullString{Valid: false},
			expected: "",
		},
		{
			name:     "empty but valid string",
			input:    sql.NullString{String: "", Valid: true},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseNullString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNullTime(t *testing.T) {
	now := time.Now()
	zeroTime := time.Time{}

	tests := []struct {
		name     string
		input    time.Time
		expected sql.NullTime
	}{
		{
			name:     "valid time",
			input:    now,
			expected: sql.NullTime{Time: now, Valid: true},
		},
		{
			name:     "zero time",
			input:    zeroTime,
			expected: sql.NullTime{Time: zeroTime, Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTime(tt.input)
			if !result.Time.Equal(tt.expected.Time) || result.Valid != tt.expected.Valid {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestNullTimePtr(t *testing.T) {
	now := time.Now()
	zeroTime := time.Time{}

	tests := []struct {
		name     string
		input    *time.Time
		expected sql.NullTime
	}{
		{
			name:     "valid time pointer",
			input:    &now,
			expected: sql.NullTime{Time: now, Valid: true},
		},
		{
			name:     "nil pointer",
			input:    nil,
			expected: sql.NullTime{Valid: false},
		},
		{
			name:     "zero time pointer",
			input:    &zeroTime,
			expected: sql.NullTime{Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTimePtr(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("Valid: expected %v, got %v", tt.expected.Valid, result.Valid)
			}
			if result.Valid && !result.Time.Equal(tt.expected.Time) {
				t.Errorf("Time: expected %v, got %v", tt.expected.Time, result.Time)
			}
		})
	}
}

func TestParseNullTimePtr(t *testing.T) {
	now := time.Now()
	zeroTime := time.Time{}

	tests := []struct {
		name     string
		input    sql.NullTime
		wantNil  bool
		expected *time.Time
	}{
		{
			name:     "valid time",
			input:    sql.NullTime{Time: now, Valid: true},
			wantNil:  false,
			expected: &now,
		},
		{
			name:     "null time",
			input:    sql.NullTime{Valid: false},
			wantNil:  true,
			expected: nil,
		},
		{
			name:     "zero time",
			input:    sql.NullTime{Time: zeroTime, Valid: true},
			wantNil:  true,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseNullTimePtr(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Error("Expected non-nil result")
				} else if !result.Equal(*tt.expected) {
					t.Errorf("Expected %v, got %v", *tt.expected, *result)
				}
			}
		})
	}
}
