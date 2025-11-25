package scanner

import (
	"errors"
	"os"
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

// errorPattern defines a pattern for error matching with its category
type errorPattern struct {
	patterns []string
	category ErrorCategory
}

// Error categorization patterns, ordered by specificity (most specific first)
var errorPatterns = []errorPattern{
	// Database errors - check first as they're most specific
	{
		patterns: []string{
			"database is locked",
			"database table is locked",
			"sqlite_busy",
			"sqlite_locked",
			"sqlite_constraint",
			"postgres",
			"pq:",
			"sql:",
			"constraint violation",
			"duplicate key",
			"unique constraint",
			"foreign key constraint",
			"violates foreign key",
			"transaction",
			"deadlock",
			"connection refused",
			"connection reset by peer",
		},
		category: ErrorCategoryDatabase,
	},
	// FFmpeg/video processing errors
	{
		patterns: []string{
			"ffmpeg",
			"ffprobe",
			"avcodec",
			"avformat",
			"moov atom not found",
			"invalid data found",
			"codec not found",
			"decoder not found",
			"encoder not found",
			"unsupported codec",
			"stream specifier",
			"no streams",
			"invalid video",
			"invalid audio",
		},
		category: ErrorCategoryFFmpeg,
	},
	// Filesystem errors - use os.PathError detection first
	{
		patterns: []string{
			"permission denied",
			"access denied",
			"no such file or directory",
			"file not found",
			"directory not found",
			"path not found",
			"read-only file system",
			"disk quota exceeded",
			"no space left on device",
			"input/output error",
			"i/o error",
			"stale file handle",
			"too many open files",
			"file name too long",
			"is a directory",
			"not a directory",
		},
		category: ErrorCategoryFilesystem,
	},
	// Metadata extraction errors (NFO, ID3, etc.)
	{
		patterns: []string{
			"nfo file",
			"nfo parse",
			"id3 tag",
			"id3v2",
			"metadata extraction",
			"invalid xml",
			"xml parse",
			"xml syntax",
			"json unmarshal",
			"json parse",
			"invalid json",
			"malformed",
		},
		category: ErrorCategoryMetadata,
	},
	// Filename parsing errors
	{
		patterns: []string{
			"could not parse filename",
			"invalid filename",
			"season number",
			"episode number",
			"track number",
			"invalid title",
			"unable to extract",
			"pattern not matched",
		},
		category: ErrorCategoryParsing,
	},
}

// Transient error patterns that indicate the error should be retried
var transientPatterns = []string{
	// Database locking
	"database is locked",
	"database table is locked",
	"sqlite_busy",
	"sqlite_locked",
	"deadlock",
	// Network/connection issues
	"connection refused",
	"connection reset",
	"connection timed out",
	"no route to host",
	"network is unreachable",
	"temporary failure",
	"resource temporarily unavailable",
	// Timeouts
	"timeout",
	"deadline exceeded",
	"context deadline exceeded",
	"i/o timeout",
	"operation timed out",
	// Temporary filesystem issues
	"stale file handle",
	"resource busy",
}

// Patterns indicating the scan job or library was deleted
var deletedPatterns = []string{
	"foreign key constraint",
	"violates foreign key",
	"fk_scan",
	"fk_library",
	"no rows in result set",
	"no such row",
	"record not found",
	"scan job not found",
	"library not found",
	"does not exist",
}

// CategorizeError determines the error category based on error type and message.
// It first checks for Go error types (os.PathError, etc.) then falls back to string matching.
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ""
	}

	// First, try to categorize by error type
	if category := categorizeByType(err); category != "" {
		return category
	}

	// Fall back to string pattern matching
	errMsg := strings.ToLower(err.Error())
	return categorizeByPattern(errMsg)
}

// categorizeByType attempts to categorize an error by its Go type
func categorizeByType(err error) ErrorCategory {
	// Check for os.PathError (filesystem errors)
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return ErrorCategoryFilesystem
	}

	// Check for os.LinkError
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return ErrorCategoryFilesystem
	}

	// Check for os.SyscallError
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		// Syscall errors are typically filesystem-related
		return ErrorCategoryFilesystem
	}

	return ""
}

// categorizeByPattern matches an error message against known patterns
func categorizeByPattern(errMsg string) ErrorCategory {
	for _, ep := range errorPatterns {
		if containsAny(errMsg, ep.patterns) {
			return ep.category
		}
	}

	// Default to parsing for unrecognized errors
	// This is a safe default as it won't trigger special handling
	return ErrorCategoryParsing
}

// IsTransientError determines if an error is transient and should be retried.
// Transient errors are temporary conditions that may resolve on retry.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for os.ErrDeadlineExceeded
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	// Check error message patterns
	errMsg := strings.ToLower(err.Error())
	return containsAny(errMsg, transientPatterns)
}

// IsScanJobDeleted checks if an error indicates the scan job or library was deleted.
// This happens when a library is deleted while a scan is running.
func IsScanJobDeleted(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return containsAny(errMsg, deletedPatterns)
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
