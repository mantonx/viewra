package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/retry"
)

// Example demonstrates basic retry usage with default configuration.
func ExampleWithRetry_basic() {
	ctx := context.Background()
	attempts := 0

	err := retry.WithRetry(ctx, retry.DefaultConfig(), func() error {
		attempts++
		if attempts < 3 {
			// Transient error - will retry
			return retry.NewRetryableError(errors.New("temporary failure"))
		}
		return nil // Success on 3rd attempt
	})

	if err != nil {
		fmt.Println("Failed:", err)
	} else {
		fmt.Println("Success after", attempts, "attempts")
	}
	// Output: Success after 3 attempts
}

// Example demonstrates custom retry configuration.
func ExampleWithRetry_customConfig() {
	ctx := context.Background()

	config := retry.Config{
		MaxRetries:     5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		Multiplier:     2.0,
	}

	err := retry.WithRetry(ctx, config, func() error {
		// Your operation here
		return nil
	})

	if err != nil {
		fmt.Println("Failed:", err)
	} else {
		fmt.Println("Success")
	}
	// Output: Success
}

// Example demonstrates handling permanent errors that should not be retried.
func ExampleWithRetry_permanentError() {
	ctx := context.Background()
	attempts := 0

	err := retry.WithRetry(ctx, retry.DefaultConfig(), func() error {
		attempts++
		// Permanent error - NOT wrapped in RetryableError
		return errors.New("file not found")
	})

	fmt.Println("Attempts:", attempts)
	fmt.Println("Error:", err)
	// Output:
	// Attempts: 1
	// Error: file not found
}

// Example demonstrates context cancellation during retry.
func ExampleWithRetry_contextCancellation() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	config := retry.Config{
		MaxRetries:     10,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
	}

	err := retry.WithRetry(ctx, config, func() error {
		return retry.NewRetryableError(errors.New("transient"))
	})

	if err != nil {
		fmt.Println("Context canceled:", errors.Is(err, context.DeadlineExceeded))
	}
	// Output: Context canceled: true
}
