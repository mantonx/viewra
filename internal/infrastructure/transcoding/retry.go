package transcoding

import (
	"context"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/retry"
)

// RetryConfig is re-exported from the retry subpackage for backward compatibility.
type RetryConfig = retry.Config

// RetryableError is re-exported from the retry subpackage for backward compatibility.
type RetryableError = retry.RetryableError

// DefaultRetryConfig returns sensible defaults for retry behavior.
// Re-exported from retry subpackage for backward compatibility.
func DefaultRetryConfig() RetryConfig {
	return retry.DefaultConfig()
}

// NewRetryableError wraps an error to indicate it's safe to retry.
// Re-exported from retry subpackage for backward compatibility.
func NewRetryableError(err error) error {
	return retry.NewRetryableError(err)
}

// IsRetryable checks if an error indicates a transient failure that should be retried.
// Re-exported from retry subpackage for backward compatibility.
func IsRetryable(err error) bool {
	return retry.IsRetryable(err)
}

// WithRetry executes a function with exponential backoff retry logic.
// Re-exported from retry subpackage for backward compatibility.
func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	return retry.WithRetry(ctx, config, fn)
}
