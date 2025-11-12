package common

import (
	"database/sql"
	"time"
)

// IsPostgres checks if the driver string indicates PostgreSQL.
// Supports: "postgres", "postgresql"
func IsPostgres(driver string) bool {
	return driver == "postgres" || driver == "postgresql"
}

// IsSQLite checks if the driver string indicates SQLite.
// Supports: "sqlite", "sqlite3", or empty (defaults to SQLite)
func IsSQLite(driver string) bool {
	return driver == "sqlite" || driver == "sqlite3" || driver == ""
}

// ParseNullTime converts sql.NullTime to time.Time
// Returns zero time if the value is NULL
func ParseNullTime(t sql.NullTime) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

// ParseTimeInterface handles various time formats from database
// SQLite can return time as string or time.Time depending on the driver
func ParseTimeInterface(t interface{}) time.Time {
	switch v := t.(type) {
	case time.Time:
		return v
	case sql.NullTime:
		return ParseNullTime(v)
	case string:
		// Try to parse common SQLite datetime formats
		layouts := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			time.RFC3339,
		}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, v); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// NullInt64 creates a sql.NullInt64 from an int64 value
// Valid is true if value is non-zero
func NullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: value,
		Valid: value > 0,
	}
}

// NullFloat64 creates a sql.NullFloat64 from a float64 value
// Valid is true if value is non-zero
func NullFloat64(value float64) sql.NullFloat64 {
	return sql.NullFloat64{
		Float64: value,
		Valid:   value > 0,
	}
}

// NullString creates a sql.NullString from a string value
// Valid is true if value is non-empty
func NullString(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}

// ParseNullString converts sql.NullString to string
// Returns empty string if the value is NULL
func ParseNullString(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// NullTime creates a sql.NullTime from a time.Time value
// Valid is true if time is not zero
func NullTime(t time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

// NullTimePtr creates a sql.NullTime from a *time.Time value
// Valid is true if pointer is not nil and time is not zero
func NullTimePtr(t *time.Time) sql.NullTime {
	if t != nil && !t.IsZero() {
		return sql.NullTime{
			Time:  *t,
			Valid: true,
		}
	}
	return sql.NullTime{Valid: false}
}

// ParseNullTimePtr converts sql.NullTime to *time.Time
// Returns nil if the value is NULL or zero
func ParseNullTimePtr(t sql.NullTime) *time.Time {
	if t.Valid && !t.Time.IsZero() {
		return &t.Time
	}
	return nil
}
