package common

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Numeric represents numeric types that can be converted to/from SQL nullable types.
type Numeric interface {
	~int32 | ~int64 | ~float64
}

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

// --- To Null Converters (Domain -> Database) ---

// NullInt32 creates a sql.NullInt32 from an int32 value.
// Valid is true if value is non-zero.
func NullInt32(value int32) sql.NullInt32 {
	return sql.NullInt32{Int32: value, Valid: value != 0}
}

// NullInt32FromInt64 creates a sql.NullInt32 from an int64 value.
// Valid is true if value is non-zero.
// Use this when converting domain int64 IDs to PostgreSQL int32 fields.
func NullInt32FromInt64(value int64) sql.NullInt32 {
	return sql.NullInt32{Int32: int32(value), Valid: value != 0}
}

// NullInt64 creates a sql.NullInt64 from an int64 value.
// Valid is true if value is non-zero.
func NullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value != 0}
}

// NullFloat64 creates a sql.NullFloat64 from a float64 value.
// Valid is true if value is non-zero.
func NullFloat64(value float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: value, Valid: value != 0}
}

// NullString creates a sql.NullString from a string value.
// Valid is true if value is non-empty.
func NullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// NullBool creates a sql.NullBool from a bool value.
// Always valid (bool has no "zero" state like numbers).
func NullBool(value bool) sql.NullBool {
	return sql.NullBool{Bool: value, Valid: true}
}

// NullTime creates a sql.NullTime from a time.Time value.
// Valid is true if time is not zero.
func NullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}

// NullTimePtr creates a sql.NullTime from a *time.Time value.
// Valid is true if pointer is not nil and time is not zero.
func NullTimePtr(t *time.Time) sql.NullTime {
	if t != nil && !t.IsZero() {
		return sql.NullTime{Time: *t, Valid: true}
	}
	return sql.NullTime{Valid: false}
}

// --- From Null Converters (Database -> Domain) ---

// ParseNullString converts sql.NullString to string.
// Returns empty string if the value is NULL.
func ParseNullString(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// ParseNullTimePtr converts sql.NullTime to *time.Time.
// Returns nil if the value is NULL or zero.
func ParseNullTimePtr(t sql.NullTime) *time.Time {
	if t.Valid && !t.Time.IsZero() {
		return &t.Time
	}
	return nil
}

// ParseNullInt64 converts sql.NullInt64 to int64.
// Returns 0 if the value is NULL.
func ParseNullInt64(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

// ParseNullInt32 converts sql.NullInt32 to int64.
// Returns 0 if the value is NULL.
func ParseNullInt32(value sql.NullInt32) int64 {
	if value.Valid {
		return int64(value.Int32)
	}
	return 0
}

// ParseNullFloat64 converts sql.NullFloat64 to float64.
// Returns 0 if the value is NULL.
func ParseNullFloat64(value sql.NullFloat64) float64 {
	if value.Valid {
		return value.Float64
	}
	return 0
}

// ParseNullBool converts sql.NullBool to bool.
// Returns false if the value is NULL.
func ParseNullBool(value sql.NullBool) bool {
	if value.Valid {
		return value.Bool
	}
	return false
}

// IsUniqueConstraintError checks if an error is a unique constraint violation
func IsUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLite - use Contains to match detailed constraint messages
	isUnique := sql.ErrNoRows != err && (strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "constraint failed"))
	if !isUnique {
		// PostgreSQL
		isUnique = strings.Contains(errStr, "duplicate key value violates unique constraint")
	}

	// DEBUG: Log the check result
	if isUnique {
		fmt.Printf("[DEBUG] IsUniqueConstraintError: TRUE for error: %v\n", errStr)
	} else if strings.Contains(errStr, "constraint") || strings.Contains(errStr, "UNIQUE") {
		fmt.Printf("[DEBUG] IsUniqueConstraintError: FALSE for error: %v\n", errStr)
	}

	return isUnique
}
