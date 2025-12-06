package transcoding

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_Success(t *testing.T) {
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

func TestWithRetry_PermanentError(t *testing.T) {
	permanentErr := errors.New("permanent failure")
	attempts := 0

	err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
		attempts++
		return permanentErr
	})

	if !errors.Is(err, permanentErr) {
		t.Errorf("expected permanent error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry for permanent error), got %d", attempts)
	}
}

func TestWithRetry_RetryableError(t *testing.T) {
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

func TestWithRetry_EventualSuccess(t *testing.T) {
	attempts := 0
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := WithRetry(context.Background(), config, func() error {
		attempts++
		if attempts < 3 {
			return NewRetryableError(errors.New("transient failure"))
		}
		return nil // Success on 3rd attempt
	})

	if err != nil {
		t.Errorf("expected success after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	attempts := 0
	err := WithRetry(ctx, DefaultRetryConfig(), func() error {
		attempts++
		return NewRetryableError(errors.New("transient failure"))
	})

	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	// Should not attempt at all since context is already canceled
	if attempts > 1 {
		t.Errorf("expected 0-1 attempts due to immediate cancellation, got %d", attempts)
	}
}

func TestWithRetry_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	attempts := 0
	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 10 * time.Millisecond, // Longer than context timeout
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := WithRetry(ctx, config, func() error {
		attempts++
		return NewRetryableError(errors.New("transient failure"))
	})

	if err == nil {
		t.Error("expected error due to context timeout")
	}

	// Should fail due to context timeout before all retries are exhausted
	if attempts > 3 {
		t.Errorf("expected few attempts before timeout, got %d", attempts)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"permanent error", errors.New("permanent"), false},
		{"retryable error", NewRetryableError(errors.New("transient")), true},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
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

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
	}

	// Test first attempt
	backoff := calculateBackoff(0, config)
	if backoff < 75*time.Millisecond || backoff > 125*time.Millisecond {
		t.Errorf("first backoff out of expected range (75-125ms): %v", backoff)
	}

	// Test second attempt (should be ~200ms ±25%)
	backoff = calculateBackoff(1, config)
	if backoff < 150*time.Millisecond || backoff > 250*time.Millisecond {
		t.Errorf("second backoff out of expected range (150-250ms): %v", backoff)
	}

	// Test that backoff is capped at MaxBackoff
	backoff = calculateBackoff(10, config)
	if backoff > config.MaxBackoff+time.Second {
		t.Errorf("backoff exceeds max backoff: %v > %v", backoff, config.MaxBackoff)
	}
}
