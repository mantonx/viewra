package scanner

import (
	"errors"
	"strings"
)

// Domain errors for scanner operations
var (
	// ErrNotFound is returned when a scan job is not found
	ErrNotFound = errors.New("scan job not found")

	// ErrInvalidPath is returned when a library path is invalid
	ErrInvalidPath = errors.New("invalid library path")

	// ErrPathNotExist is returned when a library path does not exist
	ErrPathNotExist = errors.New("library path does not exist")

	// ErrPathNotDirectory is returned when a library path is not a directory
	ErrPathNotDirectory = errors.New("library path is not a directory")

	// ErrAlreadyRunning is returned when trying to start an already running scan
	ErrAlreadyRunning = errors.New("scan already running for this library")

	// ErrNotRunning is returned when trying to pause/cancel a non-running scan
	ErrNotRunning = errors.New("scan is not running")

	// ErrInvalidStatus is returned when a status transition is invalid
	ErrInvalidStatus = errors.New("invalid scan status")
)

// CategorizeError determines the error category based on the error type and message
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ""
	}

	errMsg := strings.ToLower(err.Error())

	// Database errors
	if containsAny(errMsg, []string{
		"database",
		"sql",
		"sqlite",
		"postgres",
		"constraint",
		"duplicate",
		"unique",
		"foreign key",
		"transaction",
		"deadlock",
		"lock",
		"connection refused",
	}) {
		return ErrorCategoryDatabase
	}

	// FFmpeg/video processing errors
	if containsAny(errMsg, []string{
		"ffmpeg",
		"ffprobe",
		"codec",
		"stream",
		"invalid data",
		"moov atom",
		"decode",
		"encode",
		"unsupported format",
	}) {
		return ErrorCategoryFFmpeg
	}

	// Filesystem errors
	if containsAny(errMsg, []string{
		"permission denied",
		"no such file",
		"file not found",
		"directory not found",
		"access denied",
		"read-only",
		"disk quota",
		"no space left",
		"input/output error",
		"i/o error",
	}) {
		return ErrorCategoryFilesystem
	}

	// Metadata/parsing errors
	if containsAny(errMsg, []string{
		"parse",
		"invalid format",
		"nfo",
		"id3",
		"tag",
		"metadata",
		"xml",
		"json",
		"unmarshal",
	}) {
		return ErrorCategoryMetadata
	}

	// Filename parsing errors (specific subcategory of metadata)
	if containsAny(errMsg, []string{
		"filename",
		"season",
		"episode",
		"track number",
		"invalid title",
	}) {
		return ErrorCategoryParsing
	}

	// Default to parsing if we can't determine
	return ErrorCategoryParsing
}

// IsTransientError determines if an error is transient and should be retried
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Transient error patterns that should be retried
	transientPatterns := []string{
		"database is locked",
		"database table is locked",
		"sqlite_busy",
		"sqlite_locked",
		"temporary failure",
		"resource temporarily unavailable",
		"connection refused",
		"connection reset",
		"timeout",
		"deadline exceeded",
		"context deadline exceeded",
		"i/o timeout",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
