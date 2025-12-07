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

// Note: TestCalculateBackoff_EdgeCases has been moved to the retry subpackage
// since calculateBackoff is an internal function of that package.
