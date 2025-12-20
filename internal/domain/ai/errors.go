package ai

import "errors"

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
)
