package transcoding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestRetryableError_Unwrap tests the Unwrap method of RetryableError
func TestRetryableError_Unwrap(t *testing.T) {
	t.Run("unwrap wrapped error", func(t *testing.T) {
		originalErr := errors.New("original error")
		retryErr := &RetryableError{Err: originalErr}

		unwrapped := retryErr.Unwrap()
		if unwrapped != originalErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
		}
	})

	t.Run("unwrap nil error", func(t *testing.T) {
		retryErr := &RetryableError{Err: nil}
		unwrapped := retryErr.Unwrap()
		if unwrapped != nil {
			t.Errorf("Unwrap() = %v, want nil", unwrapped)
		}
	})

	t.Run("unwrap nested errors", func(t *testing.T) {
		innerErr := errors.New("inner error")
		wrappedErr := fmt.Errorf("wrapped: %w", innerErr)
		retryErr := &RetryableError{Err: wrappedErr}

		unwrapped := retryErr.Unwrap()
		if unwrapped != wrappedErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, wrappedErr)
		}

		// Should be able to unwrap further
		if !errors.Is(unwrapped, innerErr) {
			t.Errorf("errors.Is(unwrapped, innerErr) = false, want true")
		}
	})

	t.Run("errors.As works with Unwrap", func(t *testing.T) {
		originalErr := errors.New("test error")
		retryErr := NewRetryableError(originalErr)

		var unwrappedRetryErr *RetryableError
		if !errors.As(retryErr, &unwrappedRetryErr) {
			t.Error("errors.As() failed to match RetryableError")
		}

		if unwrappedRetryErr.Unwrap() != originalErr {
			t.Errorf("Unwrap() = %v, want %v", unwrappedRetryErr.Unwrap(), originalErr)
		}
	})
}

// TestNewRetryableError tests the NewRetryableError constructor with various error types
func TestNewRetryableError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		result := NewRetryableError(nil)
		if result != nil {
			t.Errorf("NewRetryableError(nil) = %v, want nil", result)
		}
	})

	t.Run("simple error wrapping", func(t *testing.T) {
		originalErr := errors.New("test error")
		result := NewRetryableError(originalErr)

		if result == nil {
			t.Fatal("NewRetryableError() returned nil, want error")
		}

		var retryErr *RetryableError
		if !errors.As(result, &retryErr) {
			t.Error("result is not a RetryableError")
		}

		if retryErr.Err != originalErr {
			t.Errorf("retryErr.Err = %v, want %v", retryErr.Err, originalErr)
		}
	})

	t.Run("wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		wrappedErr := fmt.Errorf("outer: %w", innerErr)
		result := NewRetryableError(wrappedErr)

		var retryErr *RetryableError
		if !errors.As(result, &retryErr) {
			t.Fatal("result is not a RetryableError")
		}

		if retryErr.Err != wrappedErr {
			t.Errorf("retryErr.Err = %v, want %v", retryErr.Err, wrappedErr)
		}

		// Should preserve the error chain
		if !errors.Is(retryErr.Err, innerErr) {
			t.Error("error chain not preserved")
		}
	})

	t.Run("already retryable error", func(t *testing.T) {
		originalErr := errors.New("original")
		firstWrap := NewRetryableError(originalErr)
		doubleWrap := NewRetryableError(firstWrap)

		// Should wrap the already-wrapped error
		var retryErr *RetryableError
		if !errors.As(doubleWrap, &retryErr) {
			t.Fatal("result is not a RetryableError")
		}

		// The wrapped error should itself be a RetryableError
		var innerRetryErr *RetryableError
		if !errors.As(retryErr.Err, &innerRetryErr) {
			t.Error("inner error is not a RetryableError")
		}

		if innerRetryErr.Err != originalErr {
			t.Errorf("innermost error = %v, want %v", innerRetryErr.Err, originalErr)
		}
	})

	t.Run("error message format", func(t *testing.T) {
		originalErr := errors.New("test message")
		result := NewRetryableError(originalErr)

		expectedMsg := "retryable error: test message"
		if result.Error() != expectedMsg {
			t.Errorf("Error() = %q, want %q", result.Error(), expectedMsg)
		}
	})

	t.Run("fmt.Errorf wrapped error", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
		result := NewRetryableError(wrappedErr)

		var retryErr *RetryableError
		if !errors.As(result, &retryErr) {
			t.Fatal("result is not a RetryableError")
		}

		// Should preserve the wrapped error
		if !errors.Is(retryErr.Err, baseErr) {
			t.Error("error chain not preserved through NewRetryableError")
		}
	})
}

// TestWithRetry_EdgeCases tests edge cases for WithRetry
func TestWithRetry_EdgeCases(t *testing.T) {
	t.Run("zero max retries", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		}

		err := WithRetry(context.Background(), config, func() error {
			attempts++
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Error("expected error, got nil")
		}

		// Should only try once (initial attempt, no retries)
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("negative max retries normalizes to zero", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     -5,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		}

		err := WithRetry(context.Background(), config, func() error {
			attempts++
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Error("expected error, got nil")
		}

		// Should normalize to 0 retries (1 attempt)
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("context canceled before first attempt", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before calling WithRetry

		attempts := 0
		err := WithRetry(ctx, DefaultRetryConfig(), func() error {
			attempts++
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Error("expected error due to context cancellation")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled in error chain, got %v", err)
		}

		// Should not attempt at all
		if attempts != 0 {
			t.Errorf("expected 0 attempts, got %d", attempts)
		}
	})

	t.Run("context canceled during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		attempts := 0
		config := RetryConfig{
			MaxRetries:     5,
			InitialBackoff: 100 * time.Millisecond, // Long backoff
			MaxBackoff:     1 * time.Second,
			Multiplier:     2.0,
		}

		// Cancel after first attempt during backoff
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := WithRetry(ctx, config, func() error {
			attempts++
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Error("expected error due to context cancellation")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled in error chain, got %v", err)
		}

		// Should stop after first attempt + canceled during backoff
		if attempts != 1 {
			t.Errorf("expected 1 attempt before cancellation, got %d", attempts)
		}

		// Error message should mention backoff
		if !strings.Contains(err.Error(), "during backoff") {
			t.Errorf("error message should mention 'during backoff', got: %v", err)
		}
	})

	t.Run("context deadline exceeded error not retried", func(t *testing.T) {
		attempts := 0
		err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
			attempts++
			return context.DeadlineExceeded
		})

		// context.DeadlineExceeded should not be retried
		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry), got %d", attempts)
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("context canceled error not retried", func(t *testing.T) {
		attempts := 0
		err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
			attempts++
			return context.Canceled
		})

		// context.Canceled should not be retried
		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry), got %d", attempts)
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("error message includes attempt count", func(t *testing.T) {
		config := RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		}

		err := WithRetry(context.Background(), config, func() error {
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Fatal("expected error")
		}

		// Should mention 3 attempts (initial + 2 retries)
		if !strings.Contains(err.Error(), "3 attempts") {
			t.Errorf("error message should mention '3 attempts', got: %v", err)
		}
	})

	t.Run("invalid config values are normalized", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     1,
			InitialBackoff: -100 * time.Millisecond, // Invalid
			MaxBackoff:     -5 * time.Second,        // Invalid
			Multiplier:     0.5,                     // Invalid (should be > 1)
		}

		// Should not panic and should use default values
		err := WithRetry(context.Background(), config, func() error {
			attempts++
			return NewRetryableError(errors.New("fail"))
		})

		if err == nil {
			t.Error("expected error")
		}

		// Should still make attempts
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("success on last allowed attempt", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		}

		err := WithRetry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return NewRetryableError(errors.New("fail"))
			}
			return nil
		})

		if err != nil {
			t.Errorf("expected success, got %v", err)
		}

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("wrapped retryable error is retried", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		}

		wrappedErr := fmt.Errorf("outer: %w", NewRetryableError(errors.New("inner")))
		err := WithRetry(context.Background(), config, func() error {
			attempts++
			return wrappedErr
		})

		if err == nil {
			t.Error("expected error")
		}

		// Should retry because the wrapped error is retryable
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})
}

// TestCalculateBackoff_EdgeCases tests edge cases for backoff calculation
func TestCalculateBackoff_EdgeCases(t *testing.T) {
	t.Run("backoff capped at max", func(t *testing.T) {
		config := RetryConfig{
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
		config := RetryConfig{
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
		config := RetryConfig{
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
		config := RetryConfig{
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
		config := RetryConfig{
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
		config := RetryConfig{
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
		config := RetryConfig{
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
