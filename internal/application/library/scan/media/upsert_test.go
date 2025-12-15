package media

import (
	"context"
	"errors"
	"testing"
)

func TestIsConstraintError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "SQLite UNIQUE constraint error",
			err:      errors.New("UNIQUE constraint failed: media.file_path"),
			expected: true,
		},
		{
			name:     "PostgreSQL duplicate key error",
			err:      errors.New("duplicate key value violates unique constraint"),
			expected: true,
		},
		{
			name:     "case sensitivity - should match",
			err:      errors.New("Error: UNIQUE constraint failed on column"),
			expected: true,
		},
		{
			name:     "PostgreSQL with details",
			err:      errors.New("pq: duplicate key value violates unique constraint \"media_file_path_key\""),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConstraintError(tt.err)
			if result != tt.expected {
				t.Errorf("IsConstraintError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestUpsertCallbacks_Fields(t *testing.T) {
	// Test that UpsertCallbacks struct can be properly initialized
	var mediaID int64 = 0

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update:     func(ctx context.Context) error { return nil },
		Create:     func(ctx context.Context) error { return nil },
		PostSave:   func(ctx context.Context) {},
	}

	// Test GetMediaID returns initial value
	if callbacks.GetMediaID() != 0 {
		t.Error("GetMediaID should return 0 initially")
	}

	// Test SetMediaID updates the value
	callbacks.SetMediaID(42)
	if mediaID != 42 {
		t.Error("SetMediaID should have set mediaID to 42")
	}

	// Test GetMediaID returns updated value
	if callbacks.GetMediaID() != 42 {
		t.Error("GetMediaID should return 42 after SetMediaID(42)")
	}
}
