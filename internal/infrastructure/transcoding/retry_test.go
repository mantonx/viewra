package transcoding

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Tests for the re-exported retry functions from the root transcoding package.
// Comprehensive retry tests are in the retry subpackage.

func TestWithRetry_ReExport(t *testing.T) {
	attempts := 0
	err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestWithRetry_RetryableError_ReExport(t *testing.T) {
	attempts := 0
	config := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := WithRetry(context.Background(), config, func() error {
		attempts++
		return NewRetryableError(errors.New("transient failure"))
	})

	if err == nil {
		t.Error("expected error after retries exhausted")
	}

	// Should be 3 total attempts: initial + 2 retries
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestIsRetryable_ReExport(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"permanent error", errors.New("permanent"), false},
		{"retryable error", NewRetryableError(errors.New("transient")), true},
		{"context canceled", context.Canceled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDefaultRetryConfig_ReExport(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", config.InitialBackoff)
	}
}
