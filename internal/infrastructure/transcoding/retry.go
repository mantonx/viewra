package transcoding

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior with exponential backoff.
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts (default: 3)
	InitialBackoff time.Duration // Initial backoff duration (default: 100ms)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 5s)
	Multiplier     float64       // Backoff multiplier (default: 2.0)
}

// DefaultRetryConfig returns sensible defaults for retry behavior.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
	}
}

// RetryableError wraps an error to indicate it should be retried.
// Transient errors (network issues, temporary resource unavailability) should use this.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError wraps an error to indicate it's safe to retry.
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// IsRetryable checks if an error indicates a transient failure that should be retried.
// Returns true for RetryableError or specific known transient errors.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if error is explicitly marked as retryable
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	// Check for context cancellation/timeout - these should NOT be retried
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Add more transient error detection here as needed
	// For now, we rely on explicit RetryableError wrapping

	return false
}

// WithRetry executes a function with exponential backoff retry logic.
// The function is retried only if it returns a RetryableError or IsRetryable returns true.
// Context cancellation is respected and stops retries immediately.
//
// Example usage:
//
//	err := WithRetry(ctx, DefaultRetryConfig(), func() error {
//	    if err := doSomething(); err != nil {
//	        if isTransient(err) {
//	            return NewRetryableError(err)
//	        }
//	        return err // Permanent error, won't retry
//	    }
//	    return nil
//	})
func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	// Validate config
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = 100 * time.Millisecond
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = 5 * time.Second
	}
	if config.Multiplier <= 1.0 {
		config.Multiplier = 2.0
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation before each attempt
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("retry aborted due to context cancellation after %d attempts: %w (last error: %v)", attempt, err, lastErr)
			}
			return fmt.Errorf("retry aborted due to context cancellation: %w", err)
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err // Permanent error, don't retry
		}

		// Don't sleep after the last attempt
		if attempt >= config.MaxRetries {
			break
		}

		// Calculate backoff with exponential growth and jitter
		backoff := calculateBackoff(attempt, config)

		// Sleep for backoff duration (respecting context cancellation)
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted during backoff after %d attempts: %w (last error: %v)", attempt+1, ctx.Err(), lastErr)
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// calculateBackoff computes the backoff duration for a given attempt.
// Uses exponential backoff with jitter to avoid thundering herd.
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential backoff: initialBackoff * multiplier^attempt
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at maximum backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter (±25% randomness to avoid thundering herd)
	jitter := backoff * 0.25 * (rand.Float64()*2 - 1) // Random value between -0.25 and +0.25
	backoff += jitter

	// Ensure backoff is positive
	if backoff < 0 {
		backoff = float64(config.InitialBackoff)
	}

	return time.Duration(backoff)
}
