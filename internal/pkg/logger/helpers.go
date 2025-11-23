package logger

import (
	"log/slog"
)

// WithTask returns a logger with task context for scheduled task execution.
// This eliminates the need to create loggers manually in each task function.
func WithTask(logger *slog.Logger, taskName string) *slog.Logger {
	return logger.With("task", taskName)
}

// WithLibrary returns a logger with library context (ID, name, path).
// Use this to avoid repeating library attributes in multiple log calls.
func WithLibrary(logger *slog.Logger, libraryID int64, name, path string) *slog.Logger {
	return logger.With(
		"library_id", libraryID,
		"name", name,
		"path", path,
	)
}

// WithLibraryID returns a logger with just the library ID.
// Use this when you only have the ID and don't need other library details.
func WithLibraryID(logger *slog.Logger, libraryID int64) *slog.Logger {
	return logger.With("library_id", libraryID)
}

// WithJob returns a logger with transcoding job context.
// Use this to avoid repeating job attributes across job lifecycle logging.
func WithJob(logger *slog.Logger, jobID, mediaID int64, quality string) *slog.Logger {
	return logger.With(
		"job_id", jobID,
		"media_id", mediaID,
		"quality", quality,
	)
}

// WithJobID returns a logger with just the job ID.
// Use this when you only have the job ID and don't need other job details.
func WithJobID(logger *slog.Logger, jobID int64) *slog.Logger {
	return logger.With("job_id", jobID)
}

// WithRequestID returns a logger with request ID context for HTTP request tracing.
// This is typically used in HTTP middleware.
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// DefaultIfNil returns the provided logger if non-nil, otherwise returns slog.Default().
// This provides a consistent fallback pattern for components that may receive a nil logger.
func DefaultIfNil(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
