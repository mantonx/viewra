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

// NullInt32 creates a sql.NullInt32 from an int32 value
// Valid is true if value is non-zero
func NullInt32(value int32) sql.NullInt32 {
	return sql.NullInt32{
		Int32: value,
		Valid: value != 0,
	}
}

// NullInt64 creates a sql.NullInt64 from an int64 value
// Valid is true if value is non-zero
func NullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: value,
		Valid: value != 0,
	}
}

// NullFloat64 creates a sql.NullFloat64 from a float64 value
// Valid is true if value is non-zero
func NullFloat64(value float64) sql.NullFloat64 {
	return sql.NullFloat64{
		Float64: value,
		Valid:   value != 0,
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

// NullBool creates a sql.NullBool from a bool value
// Always valid
func NullBool(value bool) sql.NullBool {
	return sql.NullBool{
		Bool:  value,
		Valid: true,
	}
}

// Conversion helpers for watch progress repository

// Int64ToNullInt64 converts int64 to sql.NullInt64
func Int64ToNullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: value,
		Valid: value != 0,
	}
}

// Int64ToNullInt32 converts int64 to sql.NullInt32
func Int64ToNullInt32(value int64) sql.NullInt32 {
	return sql.NullInt32{
		Int32: int32(value),
		Valid: value != 0,
	}
}

// Float64ToNullFloat64 converts float64 to sql.NullFloat64
func Float64ToNullFloat64(value float64) sql.NullFloat64 {
	return sql.NullFloat64{
		Float64: value,
		Valid:   value != 0,
	}
}

// BoolToNullBool converts bool to sql.NullBool
func BoolToNullBool(value bool) sql.NullBool {
	return sql.NullBool{
		Bool:  value,
		Valid: true,
	}
}

// TimeToNullTime converts time.Time to sql.NullTime
func TimeToNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

// NullInt64ToInt64 converts sql.NullInt64 to int64
func NullInt64ToInt64(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

// NullInt32ToInt64 converts sql.NullInt32 to int64
func NullInt32ToInt64(value sql.NullInt32) int64 {
	if value.Valid {
		return int64(value.Int32)
	}
	return 0
}

// NullFloat64ToFloat64 converts sql.NullFloat64 to float64
func NullFloat64ToFloat64(value sql.NullFloat64) float64 {
	if value.Valid {
		return value.Float64
	}
	return 0
}

// NullBoolToBool converts sql.NullBool to bool
func NullBoolToBool(value sql.NullBool) bool {
	if value.Valid {
		return value.Bool
	}
	return false
}

// NullTimeToTime converts sql.NullTime to time.Time
func NullTimeToTime(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

// IsUniqueConstraintError checks if an error is a unique constraint violation
func IsUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLite
	if sql.ErrNoRows != err && (err.Error() == "UNIQUE constraint failed" || err.Error() == "constraint failed") {
		return true
	}
	// PostgreSQL
	return errStr == "pq: duplicate key value violates unique constraint"
}
