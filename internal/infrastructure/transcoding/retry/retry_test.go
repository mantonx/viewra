package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_Success(t *testing.T) {
	attempts := 0
	err := WithRetry(context.Background(), DefaultConfig(), func() error {
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

	err := WithRetry(context.Background(), DefaultConfig(), func() error {
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
	config := Config{
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
	config := Config{
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
	err := WithRetry(ctx, DefaultConfig(), func() error {
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
	config := Config{
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
	config := Config{
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

func TestRetryableError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	retryableErr := NewRetryableError(originalErr)

	// Test that Unwrap returns the original error
	unwrapped := errors.Unwrap(retryableErr)
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}

	// Test that errors.Is works through the wrapper
	if !errors.Is(retryableErr, originalErr) {
		t.Error("errors.Is should return true for wrapped error")
	}
}

func TestNewRetryableError_Nil(t *testing.T) {
	result := NewRetryableError(nil)
	if result != nil {
		t.Errorf("NewRetryableError(nil) = %v, want nil", result)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", config.InitialBackoff)
	}
	if config.MaxBackoff != 5*time.Second {
		t.Errorf("MaxBackoff = %v, want 5s", config.MaxBackoff)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", config.Multiplier)
	}
}

func TestWithRetry_ConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"negative max retries", Config{MaxRetries: -1}},
		{"zero initial backoff", Config{InitialBackoff: 0}},
		{"negative initial backoff", Config{InitialBackoff: -1}},
		{"zero max backoff", Config{MaxBackoff: 0}},
		{"multiplier less than 1", Config{Multiplier: 0.5}},
		{"multiplier equal to 1", Config{Multiplier: 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with invalid config, just use defaults
			err := WithRetry(context.Background(), tt.config, func() error {
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error with config %+v: %v", tt.config, err)
			}
		})
	}
}

// TestCalculateBackoff_EdgeCases tests edge cases for backoff calculation
func TestCalculateBackoff_EdgeCases(t *testing.T) {
	t.Run("backoff capped at max", func(t *testing.T) {
		config := Config{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     500 * time.Millisecond,
			Multiplier:     2.0,
		}

		// High attempt number should still cap at MaxBackoff
		for attempt := 10; attempt < 20; attempt++ {
			backoff := calculateBackoff(attempt, config)
			// Allow for 25% jitter above MaxBackoff
			maxAllowed := time.Duration(float64(config.MaxBackoff) * 1.25)
			if backoff > maxAllowed {
				t.Errorf("attempt %d: backoff %v exceeds max allowed %v", attempt, backoff, maxAllowed)
			}
		}
	})

	t.Run("backoff with large multiplier", func(t *testing.T) {
		config := Config{
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     1 * time.Second,
			Multiplier:     10.0, // Large multiplier
		}

		backoff := calculateBackoff(0, config)
		if backoff < 0 {
			t.Errorf("backoff should be positive, got %v", backoff)
		}

		// Should quickly reach max
		backoff = calculateBackoff(3, config)
		maxAllowed := time.Duration(float64(config.MaxBackoff) * 1.25)
		if backoff > maxAllowed {
			t.Errorf("backoff %v exceeds max allowed %v", backoff, maxAllowed)
		}
	})

	t.Run("backoff with minimum multiplier", func(t *testing.T) {
		config := Config{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     1 * time.Second,
			Multiplier:     1.1, // Small multiplier
		}

		backoff0 := calculateBackoff(0, config)
		backoff1 := calculateBackoff(1, config)

		// Should grow slowly
		if backoff0 < 0 || backoff1 < 0 {
			t.Errorf("backoffs should be positive: %v, %v", backoff0, backoff1)
		}
	})

	t.Run("backoff is always positive", func(t *testing.T) {
		config := Config{
			InitialBackoff: 1 * time.Nanosecond, // Very small
			MaxBackoff:     1 * time.Millisecond,
			Multiplier:     2.0,
		}

		// Test many attempts to ensure jitter doesn't make it negative
		for attempt := 0; attempt < 100; attempt++ {
			backoff := calculateBackoff(attempt, config)
			if backoff < 0 {
				t.Errorf("attempt %d: backoff should be positive, got %v", attempt, backoff)
			}
		}
	})

	t.Run("backoff with zero attempt", func(t *testing.T) {
		config := Config{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
			Multiplier:     2.0,
		}

		backoff := calculateBackoff(0, config)

		// Should be around InitialBackoff ±25%
		min := time.Duration(float64(config.InitialBackoff) * 0.75)
		max := time.Duration(float64(config.InitialBackoff) * 1.25)

		if backoff < min || backoff > max {
			t.Errorf("backoff %v out of expected range [%v, %v]", backoff, min, max)
		}
	})

	t.Run("backoff has jitter variation", func(t *testing.T) {
		config := Config{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
			Multiplier:     2.0,
		}

		// Calculate backoff multiple times for same attempt
		backoffs := make(map[time.Duration]bool)
		for i := 0; i < 20; i++ {
			backoff := calculateBackoff(5, config)
			backoffs[backoff] = true
		}

		// Due to jitter, we should see variation (not always the same value)
		if len(backoffs) < 2 {
			t.Error("expected variation in backoff due to jitter, but all values were the same")
		}
	})

	t.Run("exponential growth is correct", func(t *testing.T) {
		config := Config{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
		}

		// Test that backoff roughly doubles (accounting for jitter)
		for attempt := 0; attempt < 5; attempt++ {
			backoff := calculateBackoff(attempt, config)

			// Expected value without jitter: 100ms * 2^attempt
			expectedBase := float64(config.InitialBackoff) * float64(uint(1)<<uint(attempt))

			// Allow for ±25% jitter
			min := time.Duration(expectedBase * 0.70) // A bit more tolerance
			max := time.Duration(expectedBase * 1.30)

			if backoff < min || backoff > max {
				t.Errorf("attempt %d: backoff %v out of expected range [%v, %v]",
					attempt, backoff, min, max)
			}
		}
	})
}
