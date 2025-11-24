package common

import (
	"database/sql"
	"reflect"
)

// FieldGetter is a function that extracts a field value from a row struct
// It handles the int32/int64 differences between PostgreSQL and SQLite
type FieldGetter func(row interface{}, fieldName string) interface{}

// IntFieldGetter extracts an integer field and converts it to int64
// Handles sql.NullInt32 (PostgreSQL) and sql.NullInt64 (SQLite) transparently
func IntFieldGetter(row interface{}, fieldName string) int64 {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return 0
	}

	// Handle sql.NullInt32 (PostgreSQL)
	if field.Type() == reflect.TypeOf(sql.NullInt32{}) {
		nullInt32 := field.Interface().(sql.NullInt32)
		return ParseNullInt32(nullInt32)
	}

	// Handle sql.NullInt64 (SQLite)
	if field.Type() == reflect.TypeOf(sql.NullInt64{}) {
		nullInt64 := field.Interface().(sql.NullInt64)
		return ParseNullInt64(nullInt64)
	}

	// Handle plain int32 (PostgreSQL)
	if field.Kind() == reflect.Int32 {
		return int64(field.Int())
	}

	// Handle plain int64 (SQLite)
	if field.Kind() == reflect.Int64 {
		return field.Int()
	}

	return 0
}

// NullIntFieldGetter extracts an integer field as sql.NullInt64
// Handles sql.NullInt32 (PostgreSQL) and sql.NullInt64 (SQLite) transparently
func NullIntFieldGetter(row interface{}, fieldName string) sql.NullInt64 {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return sql.NullInt64{Valid: false}
	}

	// Handle sql.NullInt32 (PostgreSQL)
	if field.Type() == reflect.TypeOf(sql.NullInt32{}) {
		nullInt32 := field.Interface().(sql.NullInt32)
		return ConvertInt32ToInt64(nullInt32)
	}

	// Handle sql.NullInt64 (SQLite)
	if field.Type() == reflect.TypeOf(sql.NullInt64{}) {
		return field.Interface().(sql.NullInt64)
	}

	// Handle plain int32 (PostgreSQL)
	if field.Kind() == reflect.Int32 {
		val := field.Int()
		return sql.NullInt64{Int64: val, Valid: val != 0}
	}

	// Handle plain int64 (SQLite)
	if field.Kind() == reflect.Int64 {
		val := field.Int()
		return sql.NullInt64{Int64: val, Valid: val != 0}
	}

	return sql.NullInt64{Valid: false}
}

// StringFieldGetter extracts a string field
func StringFieldGetter(row interface{}, fieldName string) string {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return ""
	}

	// Handle sql.NullString
	if field.Type() == reflect.TypeOf(sql.NullString{}) {
		nullString := field.Interface().(sql.NullString)
		return ParseNullString(nullString)
	}

	// Handle plain string
	if field.Kind() == reflect.String {
		return field.String()
	}

	return ""
}

// NullStringFieldGetter extracts a string field as sql.NullString
func NullStringFieldGetter(row interface{}, fieldName string) sql.NullString {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return sql.NullString{Valid: false}
	}

	// Handle sql.NullString
	if field.Type() == reflect.TypeOf(sql.NullString{}) {
		return field.Interface().(sql.NullString)
	}

	// Handle plain string
	if field.Kind() == reflect.String {
		str := field.String()
		return sql.NullString{String: str, Valid: str != ""}
	}

	return sql.NullString{Valid: false}
}

// Float64FieldGetter extracts a float64 field
func Float64FieldGetter(row interface{}, fieldName string) float64 {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return 0
	}

	// Handle sql.NullFloat64
	if field.Type() == reflect.TypeOf(sql.NullFloat64{}) {
		nullFloat := field.Interface().(sql.NullFloat64)
		return ParseNullFloat64(nullFloat)
	}

	// Handle plain float64
	if field.Kind() == reflect.Float64 {
		return field.Float()
	}

	return 0
}

// BoolFieldGetter extracts a bool field
func BoolFieldGetter(row interface{}, fieldName string) bool {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return false
	}

	// Handle sql.NullBool
	if field.Type() == reflect.TypeOf(sql.NullBool{}) {
		nullBool := field.Interface().(sql.NullBool)
		return ParseNullBool(nullBool)
	}

	// Handle plain bool
	if field.Kind() == reflect.Bool {
		return field.Bool()
	}

	return false
}

// TimeFieldGetter extracts a time field as sql.NullTime
func TimeFieldGetter(row interface{}, fieldName string) sql.NullTime {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return sql.NullTime{Valid: false}
	}

	// Handle sql.NullTime
	if field.Type() == reflect.TypeOf(sql.NullTime{}) {
		return field.Interface().(sql.NullTime)
	}

	return sql.NullTime{Valid: false}
}
