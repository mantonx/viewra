package pipeline

import "github.com/mantonx/viewra/internal/domain/enrichment"

// categorizeError attempts to categorize an error for retry logic.
func categorizeError(err error) enrichment.ErrorCategory {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// gRPC connection errors - infrastructure issue, should retry without penalty
	// These happen during plugin restarts, server restarts, or network issues
	if containsAny(errMsg,
		"connection is closing",
		"client connection is closing",
		"transport is closing",
		"connection reset",
		"broken pipe",
		"connection refused",
		"EOF",
		"context canceled",
		"DeadlineExceeded",
		"Unavailable",
		"code = Canceled",
		"code = Unavailable",
	) {
		return enrichment.ErrorCategoryNetwork
	}

	// Network errors - should retry
	if containsAny(errMsg, "timeout", "no such host", "network", "dial tcp") {
		return enrichment.ErrorCategoryNetwork
	}

	// Rate limiting - should retry with backoff
	if containsAny(errMsg, "rate limit", "429", "too many requests") {
		return enrichment.ErrorCategoryRateLimit
	}

	// Not found - probably shouldn't retry
	if containsAny(errMsg, "not found", "404") {
		return enrichment.ErrorCategoryNotFound
	}

	// Default to plugin error
	return enrichment.ErrorCategoryPlugin
}

// IsConnectionError returns true if the error is a transient connection issue
// that should not count against the retry limit.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return containsAny(errMsg,
		"connection is closing",
		"client connection is closing",
		"transport is closing",
		"connection reset",
		"broken pipe",
		"EOF",
		"context canceled",
		"code = Canceled",
		"code = Unavailable",
	)
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
