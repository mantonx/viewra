// Package errors provides structured error handling for the media module.
// It defines error types, sentinel errors, and utility functions for consistent
// error handling across the module.
package errors

import (
	"errors"
	"fmt"
)

// Error types for classification
type ErrorType string

const (
	// ErrorTypeLibrary indicates library-related errors
	ErrorTypeLibrary ErrorType = "library"
	// ErrorTypeMetadata indicates metadata-related errors
	ErrorTypeMetadata ErrorType = "metadata"
	// ErrorTypeScanning indicates scanning-related errors
	ErrorTypeScanning ErrorType = "scanning"
	// ErrorTypeFile indicates file operation errors
	ErrorTypeFile ErrorType = "file"
	// ErrorTypeDatabase indicates database operation errors
	ErrorTypeDatabase ErrorType = "database"
	// ErrorTypeValidation indicates input validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeInternal indicates internal system errors
	ErrorTypeInternal ErrorType = "internal"
)

// Sentinel errors for common scenarios
var (
	// ErrLibraryNotFound indicates a library ID doesn't exist
	ErrLibraryNotFound = errors.New("library not found")
	
	// ErrLibraryAlreadyExists indicates duplicate library creation attempt
	ErrLibraryAlreadyExists = errors.New("library already exists")
	
	// ErrMediaFileNotFound indicates a media file doesn't exist
	ErrMediaFileNotFound = errors.New("media file not found")
	
	// ErrMediaFileAlreadyExists indicates duplicate media file
	ErrMediaFileAlreadyExists = errors.New("media file already exists")
	
	// ErrInvalidPath indicates an invalid file path
	ErrInvalidPath = errors.New("invalid path")
	
	// ErrPathNotAccessible indicates path cannot be accessed
	ErrPathNotAccessible = errors.New("path not accessible")
	
	// ErrScanInProgress indicates a scan is already running
	ErrScanInProgress = errors.New("scan already in progress")
	
	// ErrMetadataNotFound indicates metadata doesn't exist
	ErrMetadataNotFound = errors.New("metadata not found")
	
	// ErrInvalidMediaType indicates unsupported media type
	ErrInvalidMediaType = errors.New("invalid media type")
	
	// ErrInvalidInput indicates invalid request parameters
	ErrInvalidInput = errors.New("invalid input")
	
	// ErrDatabaseOperation indicates a database operation failed
	ErrDatabaseOperation = errors.New("database operation failed")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrCancelled indicates an operation was cancelled
	ErrCancelled = errors.New("operation cancelled")
)

// MediaError provides structured error information with context
type MediaError struct {
	Type       ErrorType              // Error classification
	Op         string                 // Operation that failed (e.g., "create_library", "scan")
	LibraryID  string                 // Related library ID if applicable
	MediaID    string                 // Related media file ID if applicable
	Path       string                 // Related file path if applicable
	Err        error                  // Underlying error
	Details    map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *MediaError) Error() string {
	var context []string
	
	if e.LibraryID != "" {
		context = append(context, fmt.Sprintf("library=%s", e.LibraryID))
	}
	if e.MediaID != "" {
		context = append(context, fmt.Sprintf("media=%s", e.MediaID))
	}
	if e.Path != "" {
		context = append(context, fmt.Sprintf("path=%s", e.Path))
	}
	
	if len(context) > 0 {
		return fmt.Sprintf("%s error in %s [%s]: %v", e.Type, e.Op, context[0], e.Err)
	}
	return fmt.Sprintf("%s error in %s: %v", e.Type, e.Op, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *MediaError) Unwrap() error {
	return e.Err
}

// Is implements error comparison for sentinel errors
func (e *MediaError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// New creates a new MediaError
func New(errType ErrorType, op string, err error) *MediaError {
	return &MediaError{
		Type:    errType,
		Op:      op,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// WithLibrary adds library context to the error
func (e *MediaError) WithLibrary(libraryID string) *MediaError {
	e.LibraryID = libraryID
	return e
}

// WithMedia adds media file context to the error
func (e *MediaError) WithMedia(mediaID string) *MediaError {
	e.MediaID = mediaID
	return e
}

// WithPath adds file path context to the error
func (e *MediaError) WithPath(path string) *MediaError {
	e.Path = path
	return e
}

// WithDetail adds a key-value detail to the error
func (e *MediaError) WithDetail(key string, value interface{}) *MediaError {
	e.Details[key] = value
	return e
}

// WithDetails adds multiple key-value details to the error
func (e *MediaError) WithDetails(details map[string]interface{}) *MediaError {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// IsRecoverable returns true if the error might succeed on retry
func (e *MediaError) IsRecoverable() bool {
	// Timeout errors are often recoverable
	if errors.Is(e.Err, ErrTimeout) {
		return true
	}
	
	// Scan in progress might clear up
	if errors.Is(e.Err, ErrScanInProgress) {
		return true
	}
	
	// Database errors might be transient
	if e.Type == ErrorTypeDatabase {
		return true
	}
	
	// Internal errors might be transient
	if e.Type == ErrorTypeInternal {
		return true
	}
	
	return false
}

// Error creation helpers

// LibraryError creates a library-related error
func LibraryError(op string, err error) *MediaError {
	return New(ErrorTypeLibrary, op, err)
}

// MetadataError creates a metadata-related error
func MetadataError(op string, err error) *MediaError {
	return New(ErrorTypeMetadata, op, err)
}

// ScanningError creates a scanning-related error
func ScanningError(op string, err error) *MediaError {
	return New(ErrorTypeScanning, op, err)
}

// FileError creates a file operation error
func FileError(op string, err error) *MediaError {
	return New(ErrorTypeFile, op, err)
}

// DatabaseError creates a database operation error
func DatabaseError(op string, err error) *MediaError {
	return New(ErrorTypeDatabase, op, err)
}

// ValidationError creates a validation error
func ValidationError(op string, err error) *MediaError {
	return New(ErrorTypeValidation, op, err)
}

// InternalError creates an internal system error
func InternalError(op string, err error) *MediaError {
	return New(ErrorTypeInternal, op, err)
}

// Wrap wraps an error with operation context if it's not already a MediaError
func Wrap(err error, errType ErrorType, op string) error {
	if err == nil {
		return nil
	}
	
	// If it's already a MediaError, preserve it
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return err
	}
	
	// Otherwise wrap it
	return New(errType, op, err)
}

// GetType extracts the error type from an error
func GetType(err error) ErrorType {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.Type
	}
	return ErrorTypeInternal
}

// GetOperation extracts the operation from an error
func GetOperation(err error) string {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.Op
	}
	return "unknown"
}

// GetLibraryID extracts the library ID from an error
func GetLibraryID(err error) string {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.LibraryID
	}
	return ""
}

// GetMediaID extracts the media ID from an error
func GetMediaID(err error) string {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.MediaID
	}
	return ""
}

// GetPath extracts the file path from an error
func GetPath(err error) string {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.Path
	}
	return ""
}

// GetDetails extracts error details
func GetDetails(err error) map[string]interface{} {
	var mErr *MediaError
	if errors.As(err, &mErr) {
		return mErr.Details
	}
	return nil
}