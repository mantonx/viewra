// Package errors provides structured error handling for the transcoding module.
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
	// ErrorTypeSession indicates session-related errors
	ErrorTypeSession ErrorType = "session"
	// ErrorTypeStorage indicates storage-related errors
	ErrorTypeStorage ErrorType = "storage"
	// ErrorTypeProvider indicates provider-related errors
	ErrorTypeProvider ErrorType = "provider"
	// ErrorTypeResource indicates resource-related errors (limits, quotas)
	ErrorTypeResource ErrorType = "resource"
	// ErrorTypeValidation indicates input validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeTranscode indicates transcoding operation errors
	ErrorTypeTranscode ErrorType = "transcode"
	// ErrorTypeInternal indicates internal system errors
	ErrorTypeInternal ErrorType = "internal"
)

// Sentinel errors for common scenarios
var (
	// ErrSessionNotFound indicates a session ID doesn't exist
	ErrSessionNotFound = errors.New("session not found")
	
	// ErrSessionAlreadyExists indicates duplicate session creation attempt
	ErrSessionAlreadyExists = errors.New("session already exists")
	
	// ErrProviderNotFound indicates a provider ID doesn't exist
	ErrProviderNotFound = errors.New("provider not found")
	
	// ErrProviderNotAvailable indicates a provider is registered but not available
	ErrProviderNotAvailable = errors.New("provider not available")
	
	// ErrNoProvidersAvailable indicates no providers support the requested format
	ErrNoProvidersAvailable = errors.New("no providers available for format")
	
	// ErrContentNotFound indicates content hash doesn't exist in storage
	ErrContentNotFound = errors.New("content not found")
	
	// ErrResourceLimitExceeded indicates max concurrent sessions reached
	ErrResourceLimitExceeded = errors.New("resource limit exceeded")
	
	// ErrQueueFull indicates the transcoding queue is full
	ErrQueueFull = errors.New("transcoding queue full")
	
	// ErrInvalidInput indicates invalid request parameters
	ErrInvalidInput = errors.New("invalid input")
	
	// ErrTranscodeFailed indicates the transcoding operation failed
	ErrTranscodeFailed = errors.New("transcode failed")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrCancelled indicates an operation was cancelled
	ErrCancelled = errors.New("operation cancelled")
)

// TranscodingError provides structured error information with context
type TranscodingError struct {
	Type      ErrorType              // Error classification
	Op        string                 // Operation that failed (e.g., "create_session", "transcode")
	SessionID string                 // Related session ID if applicable
	Err       error                  // Underlying error
	Details   map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *TranscodingError) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("%s error in %s for session %s: %v", e.Type, e.Op, e.SessionID, e.Err)
	}
	return fmt.Sprintf("%s error in %s: %v", e.Type, e.Op, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *TranscodingError) Unwrap() error {
	return e.Err
}

// Is implements error comparison for sentinel errors
func (e *TranscodingError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// New creates a new TranscodingError
func New(errType ErrorType, op string, err error) *TranscodingError {
	return &TranscodingError{
		Type:    errType,
		Op:      op,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// WithSession adds session context to the error
func (e *TranscodingError) WithSession(sessionID string) *TranscodingError {
	e.SessionID = sessionID
	return e
}

// WithDetail adds a key-value detail to the error
func (e *TranscodingError) WithDetail(key string, value interface{}) *TranscodingError {
	e.Details[key] = value
	return e
}

// WithDetails adds multiple key-value details to the error
func (e *TranscodingError) WithDetails(details map[string]interface{}) *TranscodingError {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// IsRecoverable returns true if the error might succeed on retry
func (e *TranscodingError) IsRecoverable() bool {
	// Timeout and resource errors are often recoverable
	if errors.Is(e.Err, ErrTimeout) || errors.Is(e.Err, ErrResourceLimitExceeded) || errors.Is(e.Err, ErrQueueFull) {
		return true
	}
	
	// Provider availability might be temporary
	if errors.Is(e.Err, ErrProviderNotAvailable) {
		return true
	}
	
	// Internal errors might be transient
	if e.Type == ErrorTypeInternal {
		return true
	}
	
	return false
}

// Error creation helpers

// SessionError creates a session-related error
func SessionError(op string, err error) *TranscodingError {
	return New(ErrorTypeSession, op, err)
}

// StorageError creates a storage-related error
func StorageError(op string, err error) *TranscodingError {
	return New(ErrorTypeStorage, op, err)
}

// ProviderError creates a provider-related error
func ProviderError(op string, err error) *TranscodingError {
	return New(ErrorTypeProvider, op, err)
}

// ResourceError creates a resource-related error
func ResourceError(op string, err error) *TranscodingError {
	return New(ErrorTypeResource, op, err)
}

// ValidationError creates a validation error
func ValidationError(op string, err error) *TranscodingError {
	return New(ErrorTypeValidation, op, err)
}

// TranscodeError creates a transcoding operation error
func TranscodeError(op string, err error) *TranscodingError {
	return New(ErrorTypeTranscode, op, err)
}

// InternalError creates an internal system error
func InternalError(op string, err error) *TranscodingError {
	return New(ErrorTypeInternal, op, err)
}

// Wrap wraps an error with operation context if it's not already a TranscodingError
func Wrap(err error, errType ErrorType, op string) error {
	if err == nil {
		return nil
	}
	
	// If it's already a TranscodingError, preserve it
	var tErr *TranscodingError
	if errors.As(err, &tErr) {
		return err
	}
	
	// Otherwise wrap it
	return New(errType, op, err)
}

// GetType extracts the error type from an error
func GetType(err error) ErrorType {
	var tErr *TranscodingError
	if errors.As(err, &tErr) {
		return tErr.Type
	}
	return ErrorTypeInternal
}

// GetOperation extracts the operation from an error
func GetOperation(err error) string {
	var tErr *TranscodingError
	if errors.As(err, &tErr) {
		return tErr.Op
	}
	return "unknown"
}

// GetSessionID extracts the session ID from an error
func GetSessionID(err error) string {
	var tErr *TranscodingError
	if errors.As(err, &tErr) {
		return tErr.SessionID
	}
	return ""
}

// GetDetails extracts error details
func GetDetails(err error) map[string]interface{} {
	var tErr *TranscodingError
	if errors.As(err, &tErr) {
		return tErr.Details
	}
	return nil
}