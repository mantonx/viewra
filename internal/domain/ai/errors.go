package ai

import (
	"errors"
	"fmt"
)

var (
	// ErrProviderNotConfigured is returned when the AI provider is not configured.
	ErrProviderNotConfigured = errors.New("ai provider not configured")

	// ErrProviderUnavailable is returned when the AI provider is not reachable.
	ErrProviderUnavailable = errors.New("ai provider unavailable")

	// ErrInvalidAPIKey is returned when the API key is invalid or expired.
	ErrInvalidAPIKey = errors.New("invalid or expired API key")

	// ErrRateLimitExceeded is returned when rate limits are exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrDailyLimitExceeded is returned when daily token limits are exceeded.
	ErrDailyLimitExceeded = errors.New("daily token limit exceeded")

	// ErrMonthlyBudgetExceeded is returned when monthly budget is exceeded.
	ErrMonthlyBudgetExceeded = errors.New("monthly budget exceeded")

	// ErrModelNotFound is returned when the requested model is not available.
	ErrModelNotFound = errors.New("model not found")

	// ErrContextTooLong is returned when the input exceeds the model's context window.
	ErrContextTooLong = errors.New("context too long for model")

	// ErrEmbeddingFailed is returned when embedding generation fails.
	ErrEmbeddingFailed = errors.New("embedding generation failed")

	// ErrVectorStorageNotConfigured is returned when vector storage is not set up.
	ErrVectorStorageNotConfigured = errors.New("vector storage not configured")

	// ErrIndexingInProgress is returned when an indexing operation is already running.
	ErrIndexingInProgress = errors.New("indexing already in progress")

	// ErrNoEmbeddingsFound is returned when no embeddings exist for semantic search.
	ErrNoEmbeddingsFound = errors.New("no embeddings found, please run indexing first")

	// ErrUnsupportedEntityType is returned when the entity type is not supported.
	ErrUnsupportedEntityType = errors.New("unsupported entity type")
)

// ProviderError contains detailed error information from provider SDKs.
// It wraps the underlying SDK error with additional context for debugging
// and user-facing error messages.
type ProviderError struct {
	// Code is a machine-readable error code (e.g., "rate_limit_exceeded", "invalid_api_key")
	Code string `json:"code"`

	// Message is a human-readable error message from the provider
	Message string `json:"message"`

	// Provider identifies which provider returned the error (e.g., "openai", "anthropic")
	Provider string `json:"provider"`

	// StatusCode is the HTTP status code if applicable
	StatusCode int `json:"statusCode,omitempty"`

	// RetryAfter indicates when to retry for rate limit errors (e.g., "60s")
	RetryAfter string `json:"retryAfter,omitempty"`

	// Cause is the original SDK error for debugging
	Cause error `json:"-"`
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Provider, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Code)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a new ProviderError with the given details.
func NewProviderError(provider, code, message string, statusCode int, cause error) *ProviderError {
	return &ProviderError{
		Code:       code,
		Message:    message,
		Provider:   provider,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// IsRateLimited returns true if the error is a rate limit error.
func IsRateLimited(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Code == "rate_limit_exceeded" || pe.StatusCode == 429
	}
	return errors.Is(err, ErrRateLimitExceeded)
}

// IsInvalidAPIKey returns true if the error is an invalid API key error.
func IsInvalidAPIKey(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Code == "invalid_api_key" || pe.StatusCode == 401
	}
	return errors.Is(err, ErrInvalidAPIKey)
}

// IsModelNotFound returns true if the error is a model not found error.
func IsModelNotFound(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Code == "model_not_found" || pe.StatusCode == 404
	}
	return errors.Is(err, ErrModelNotFound)
}

// IsProviderUnavailable returns true if the error indicates the provider is unreachable.
func IsProviderUnavailable(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Code == "provider_unavailable" || pe.StatusCode >= 500
	}
	return errors.Is(err, ErrProviderUnavailable)
}
