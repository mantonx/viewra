package adapters

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

// TypeAdapter handles conversions between database-specific types.
// It uses reflection to automatically copy fields between similar structs
// and handles common type conversions like int32 ↔ int64.
type TypeAdapter struct{}

// NewTypeAdapter creates a new TypeAdapter instance.
func NewTypeAdapter() *TypeAdapter {
	return &TypeAdapter{}
}

// ConvertStruct copies fields from src to dst, handling common type conversions.
// It automatically handles:
//   - int32 ↔ int64 conversions (PostgreSQL vs SQLite)
//   - sql.NullInt32 ↔ sql.NullInt64
//   - sql.NullString, sql.NullBool, sql.NullFloat64 (passthrough)
//   - time.Time and interface{} conversions
//
// Fields that exist in dst but not in src are left unchanged.
// Fields that cannot be converted return an error.
func (a *TypeAdapter) ConvertStruct(src, dst interface{}) error {
	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst)

	// Handle pointers
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}

	if !dstVal.CanSet() {
		return fmt.Errorf("destination is not settable")
	}

	// Iterate through destination fields
	for i := 0; i < dstVal.NumField(); i++ {
		dstField := dstVal.Field(i)
		dstFieldName := dstVal.Type().Field(i).Name

		// Skip unexported fields
		if !dstField.CanSet() {
			continue
		}

		// Find matching source field
		srcField := srcVal.FieldByName(dstFieldName)
		if !srcField.IsValid() {
			continue // Skip fields that don't exist in source
		}

		// Convert and set field
		if err := a.setField(dstField, srcField); err != nil {
			return fmt.Errorf("failed to set field %s: %w", dstFieldName, err)
		}
	}

	return nil
}

// setField converts and sets a single field value.
func (a *TypeAdapter) setField(dst, src reflect.Value) error {
	// Same type - direct assignment
	if dst.Type() == src.Type() {
		dst.Set(src)
		return nil
	}

	// int32 ↔ int64 conversions (Postgres ↔ SQLite)
	if a.isIntType(dst.Type()) && a.isIntType(src.Type()) {
		dst.SetInt(src.Int())
		return nil
	}

	// sql.NullInt32 ↔ sql.NullInt64
	if dst.Type() == reflect.TypeOf(sql.NullInt64{}) && src.Type() == reflect.TypeOf(sql.NullInt32{}) {
		nullInt32 := src.Interface().(sql.NullInt32)
		dst.Set(reflect.ValueOf(sql.NullInt64{
			Int64: int64(nullInt32.Int32),
			Valid: nullInt32.Valid,
		}))
		return nil
	}
	if dst.Type() == reflect.TypeOf(sql.NullInt32{}) && src.Type() == reflect.TypeOf(sql.NullInt64{}) {
		nullInt64 := src.Interface().(sql.NullInt64)
		dst.Set(reflect.ValueOf(sql.NullInt32{
			Int32: int32(nullInt64.Int64),
			Valid: nullInt64.Valid,
		}))
		return nil
	}

	// Time conversions (interface{} to time.Time)
	if dst.Type() == reflect.TypeOf(time.Time{}) {
		if t, ok := src.Interface().(time.Time); ok {
			dst.Set(reflect.ValueOf(t))
			return nil
		}
		// Handle string timestamps if needed
		if str, ok := src.Interface().(string); ok && str != "" {
			t, err := time.Parse(time.RFC3339, str)
			if err != nil {
				return fmt.Errorf("cannot parse time string: %w", err)
			}
			dst.Set(reflect.ValueOf(t))
			return nil
		}
	}

	// interface{} to time.Time (for SQLite TEXT timestamps)
	if src.Kind() == reflect.Interface && dst.Type() == reflect.TypeOf(time.Time{}) {
		if t, ok := src.Interface().(time.Time); ok {
			dst.Set(reflect.ValueOf(t))
			return nil
		}
	}

	// time.Time to interface{}
	if src.Type() == reflect.TypeOf(time.Time{}) && dst.Kind() == reflect.Interface {
		dst.Set(src)
		return nil
	}

	// sql.NullTime handling
	if dst.Type() == reflect.TypeOf(sql.NullTime{}) && src.Type() == reflect.TypeOf(sql.NullTime{}) {
		dst.Set(src)
		return nil
	}

	// Default: try to convert
	if src.Type().ConvertibleTo(dst.Type()) {
		dst.Set(src.Convert(dst.Type()))
		return nil
	}

	return fmt.Errorf("cannot convert %s to %s", src.Type(), dst.Type())
}

// isIntType checks if a type is an integer type.
func (a *TypeAdapter) isIntType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

// ConvertSlice converts a slice of structs from type T to type U.
// This is a convenience method for converting query results.
func ConvertSlice[T any, U any](src []T, adapter *TypeAdapter) ([]U, error) {
	result := make([]U, len(src))
	for i, item := range src {
		if err := adapter.ConvertStruct(item, &result[i]); err != nil {
			return nil, fmt.Errorf("failed to convert slice item %d: %w", i, err)
		}
	}
	return result, nil
}
