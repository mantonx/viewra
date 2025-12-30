package common

import (
	"database/sql"
	"log/slog"
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

// NullInt64Ptr creates a sql.NullInt64 from an *int64 pointer.
// Valid is true if pointer is non-nil.
func NullInt64Ptr(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

// NullInt64FromIntPtr creates a sql.NullInt64 from an *int pointer.
// Valid is true if pointer is non-nil.
func NullInt64FromIntPtr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

// NullInt32FromIntPtr creates a sql.NullInt32 from an *int pointer.
// Valid is true if pointer is non-nil.
func NullInt32FromIntPtr(value *int) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Int32: int32(*value), Valid: true}
}

// NullInt32Ptr creates a sql.NullInt32 from an *int64 pointer.
// Valid is true if pointer is non-nil.
func NullInt32Ptr(value *int64) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Int32: int32(*value), Valid: true}
}

// NullFloat64 creates a sql.NullFloat64 from a float64 value.
// Valid is true if value is non-zero.
func NullFloat64(value float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: value, Valid: value != 0}
}

// NullFloat64FromFloat32 creates a sql.NullFloat64 from a float32 value.
// Valid is true if value is non-zero.
func NullFloat64FromFloat32(value float32) sql.NullFloat64 {
	return sql.NullFloat64{Float64: float64(value), Valid: value != 0}
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

// NullInt64FromBool creates a sql.NullInt64 from a bool value.
// Used for SQLite where booleans are stored as INTEGER (0/1).
// Always valid (bool has no "zero" state like numbers).
func NullInt64FromBool(value bool) sql.NullInt64 {
	var intVal int64
	if value {
		intVal = 1
	}
	return sql.NullInt64{Int64: intVal, Valid: true}
}

// BoolToInt64 converts a bool to int64 (0 or 1).
// Used for SQLite where booleans are stored as INTEGER.
func BoolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

// Int64ToBool converts an int64 to bool.
// Non-zero values are true, zero is false.
func Int64ToBool(value int64) bool {
	return value != 0
}

// NullInt64ToBool converts sql.NullInt64 to bool.
// Returns false if the value is NULL or zero.
func NullInt64ToBool(value sql.NullInt64) bool {
	return value.Valid && value.Int64 != 0
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

// ParseNullInt64Ptr converts sql.NullInt64 to *int64.
// Returns nil if the value is NULL.
func ParseNullInt64Ptr(value sql.NullInt64) *int64 {
	if value.Valid {
		val := value.Int64
		return &val
	}
	return nil
}

// ParseNullInt64ToIntPtr converts sql.NullInt64 to *int.
// Returns nil if the value is NULL.
func ParseNullInt64ToIntPtr(value sql.NullInt64) *int {
	if value.Valid {
		val := int(value.Int64)
		return &val
	}
	return nil
}

// ParseNullInt32ToIntPtr converts sql.NullInt32 to *int.
// Returns nil if the value is NULL.
func ParseNullInt32ToIntPtr(value sql.NullInt32) *int {
	if value.Valid {
		val := int(value.Int32)
		return &val
	}
	return nil
}

// ParseNullInt32Ptr converts sql.NullInt32 to *int64.
// Returns nil if the value is NULL.
// Deprecated: Use ParseNullInt64Ptr instead, as all integer types are now int64.
func ParseNullInt32Ptr(value sql.NullInt32) *int64 {
	if value.Valid {
		val := int64(value.Int32)
		return &val
	}
	return nil
}

// NullInt64PtrFromInt64 creates a sql.NullInt64 from an *int64 pointer.
// Valid is true if pointer is non-nil.
// Alias for NullInt64Ptr for clarity when replacing NullInt32Ptr usage.
func NullInt64PtrFromInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

// ParseNullInt32 converts sql.NullInt32 to int64.
// Returns 0 if the value is NULL.
func ParseNullInt32(value sql.NullInt32) int64 {
	if value.Valid {
		return int64(value.Int32)
	}
	return 0
}

// ConvertInt32ToInt64 converts sql.NullInt32 to sql.NullInt64.
// Used when converting PostgreSQL results to domain types.
func ConvertInt32ToInt64(value sql.NullInt32) sql.NullInt64 {
	if value.Valid {
		return sql.NullInt64{Int64: int64(value.Int32), Valid: true}
	}
	return sql.NullInt64{Valid: false}
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
		slog.Debug("unique constraint error detected", "error", errStr)
	} else if strings.Contains(errStr, "constraint") || strings.Contains(errStr, "UNIQUE") {
		slog.Debug("constraint error but not unique", "error", errStr)
	}

	return isUnique
}

// FormatNullDate converts sql.NullTime to ISO 8601 date string (YYYY-MM-DD).
// Returns empty string if the value is NULL or zero.
func FormatNullDate(t sql.NullTime) string {
	if t.Valid && !t.Time.IsZero() {
		return t.Time.Format("2006-01-02")
	}
	return ""
}

// ParseDateString parses an ISO 8601 date string (YYYY-MM-DD) to sql.NullTime.
// Returns NULL NullTime if the string is empty or invalid.
func ParseDateString(s string) sql.NullTime {
	if s == "" {
		return sql.NullTime{Valid: false}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// NullStringPtr creates a sql.NullString from a *string pointer.
// Valid is true if pointer is non-nil and non-empty.
func NullStringPtr(value *string) sql.NullString {
	if value == nil || *value == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *value, Valid: true}
}

// ParseNullStringPtr converts sql.NullString to *string.
// Returns nil if the value is NULL.
func ParseNullStringPtr(s sql.NullString) *string {
	if s.Valid {
		return &s.String
	}
	return nil
}

// NullFloat64Ptr creates a sql.NullFloat64 from a *float64 pointer.
// Valid is true if pointer is non-nil.
func NullFloat64Ptr(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

// NullFloat32Ptr creates a sql.NullFloat64 from a *float64 pointer for PostgreSQL float32 fields.
// Valid is true if pointer is non-nil.
func NullFloat32Ptr(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

// --- Interface Converters (for aggregate query results) ---

// Float64FromInterface extracts a float64 from an interface{}.
// Handles various numeric types that databases return for AVG/SUM.
func Float64FromInterface(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case int:
		return float64(val)
	case []byte:
		// SQLite may return numeric strings as []byte
		var f float64
		if _, err := parseNumber(string(val), &f); err == nil {
			return f
		}
	case string:
		var f float64
		if _, err := parseNumber(val, &f); err == nil {
			return f
		}
	}
	return 0
}

// Int64FromInterface extracts an int64 from an interface{}.
// Handles various numeric types that databases return for COUNT/SUM.
func Int64FromInterface(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	case []byte:
		var i int64
		if _, err := parseNumber(string(val), &i); err == nil {
			return i
		}
	case string:
		var i int64
		if _, err := parseNumber(val, &i); err == nil {
			return i
		}
	}
	return 0
}

// Int64PtrFromInterface extracts an *int64 from an interface{}.
// Returns nil if the value is nil or cannot be converted.
func Int64PtrFromInterface(v interface{}) *int64 {
	if v == nil {
		return nil
	}
	val := Int64FromInterface(v)
	return &val
}

// parseNumber is a helper that parses a string to a numeric type.
func parseNumber[T Numeric](s string, target *T) (T, error) {
	var result T
	switch any(target).(type) {
	case *float64:
		var f float64
		parseFloat(s, &f)
		*target = T(f)
		return *target, nil
	case *int64:
		var i int64
		parseInt(s, &i)
		*target = T(i)
		return *target, nil
	case *int32:
		var i int64
		parseInt(s, &i)
		*target = T(int32(i))
		return *target, nil
	}
	return result, nil
}

func parseFloat(s string, f *float64) {
	// Strip non-numeric characters except . and -
	clean := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == '-' || (c >= '0' && c <= '9') {
			clean = append(clean, c)
		} else if len(clean) > 0 {
			break
		}
	}
	s = string(clean)
	if len(s) == 0 {
		*f = 0
		return
	}

	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}

	var val float64
	decimalPos := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			decimalPos = i
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			break
		}
		val = val*10 + float64(s[i]-'0')
	}

	if decimalPos >= 0 {
		divisor := 1.0
		for i := 0; i < len(s)-decimalPos-1; i++ {
			divisor *= 10
		}
		val /= divisor
	}

	if neg {
		val = -val
	}
	*f = val
}

func parseInt(s string, i *int64) {
	var val int64
	neg := false
	for idx := 0; idx < len(s); idx++ {
		if s[idx] == '-' && idx == 0 {
			neg = true
			continue
		}
		if s[idx] < '0' || s[idx] > '9' {
			break
		}
		val = val*10 + int64(s[idx]-'0')
	}
	if neg {
		val = -val
	}
	*i = val
}
