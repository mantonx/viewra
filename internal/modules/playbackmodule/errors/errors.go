// Package errors provides structured error handling for the playback module.
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
	// ErrorTypeDecision indicates playback decision errors
	ErrorTypeDecision ErrorType = "decision"
	// ErrorTypeSession indicates session-related errors
	ErrorTypeSession ErrorType = "session"
	// ErrorTypeStreaming indicates streaming-related errors
	ErrorTypeStreaming ErrorType = "streaming"
	// ErrorTypeHistory indicates history tracking errors
	ErrorTypeHistory ErrorType = "history"
	// ErrorTypeCleanup indicates cleanup operation errors
	ErrorTypeCleanup ErrorType = "cleanup"
	// ErrorTypeDevice indicates device profile errors
	ErrorTypeDevice ErrorType = "device"
	// ErrorTypeValidation indicates input validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeInternal indicates internal system errors
	ErrorTypeInternal ErrorType = "internal"
)

// Sentinel errors for common scenarios
var (
	// ErrSessionNotFound indicates a session ID doesn't exist
	ErrSessionNotFound = errors.New("session not found")
	
	// ErrSessionAlreadyExists indicates duplicate session creation attempt
	ErrSessionAlreadyExists = errors.New("session already exists")
	
	// ErrSessionExpired indicates a session has expired
	ErrSessionExpired = errors.New("session expired")
	
	// ErrInvalidDeviceProfile indicates unsupported device profile
	ErrInvalidDeviceProfile = errors.New("invalid device profile")
	
	// ErrNoSuitableMethod indicates no playback method available
	ErrNoSuitableMethod = errors.New("no suitable playback method")
	
	// ErrStreamNotReady indicates stream is not ready for playback
	ErrStreamNotReady = errors.New("stream not ready")
	
	// ErrStreamInterrupted indicates stream was interrupted
	ErrStreamInterrupted = errors.New("stream interrupted")
	
	// ErrHistoryNotFound indicates playback history doesn't exist
	ErrHistoryNotFound = errors.New("history not found")
	
	// ErrCleanupInProgress indicates cleanup is already running
	ErrCleanupInProgress = errors.New("cleanup already in progress")
	
	// ErrInvalidInput indicates invalid request parameters
	ErrInvalidInput = errors.New("invalid input")
	
	// ErrMediaNotFound indicates media file doesn't exist
	ErrMediaNotFound = errors.New("media not found")
	
	// ErrTranscodeRequired indicates transcoding is needed
	ErrTranscodeRequired = errors.New("transcode required")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrCancelled indicates an operation was cancelled
	ErrCancelled = errors.New("operation cancelled")
)

// PlaybackError provides structured error information with context
type PlaybackError struct {
	Type       ErrorType              // Error classification
	Op         string                 // Operation that failed (e.g., "decide_playback", "create_session")
	SessionID  string                 // Related session ID if applicable
	MediaID    string                 // Related media file ID if applicable
	UserID     string                 // Related user ID if applicable
	DeviceID   string                 // Related device ID if applicable
	Err        error                  // Underlying error
	Details    map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *PlaybackError) Error() string {
	var context []string
	
	if e.SessionID != "" {
		context = append(context, fmt.Sprintf("session=%s", e.SessionID))
	}
	if e.MediaID != "" {
		context = append(context, fmt.Sprintf("media=%s", e.MediaID))
	}
	if e.UserID != "" {
		context = append(context, fmt.Sprintf("user=%s", e.UserID))
	}
	if e.DeviceID != "" {
		context = append(context, fmt.Sprintf("device=%s", e.DeviceID))
	}
	
	if len(context) > 0 {
		return fmt.Sprintf("%s error in %s [%s]: %v", e.Type, e.Op, context[0], e.Err)
	}
	return fmt.Sprintf("%s error in %s: %v", e.Type, e.Op, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *PlaybackError) Unwrap() error {
	return e.Err
}

// Is implements error comparison for sentinel errors
func (e *PlaybackError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// New creates a new PlaybackError
func New(errType ErrorType, op string, err error) *PlaybackError {
	return &PlaybackError{
		Type:    errType,
		Op:      op,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// WithSession adds session context to the error
func (e *PlaybackError) WithSession(sessionID string) *PlaybackError {
	e.SessionID = sessionID
	return e
}

// WithMedia adds media file context to the error
func (e *PlaybackError) WithMedia(mediaID string) *PlaybackError {
	e.MediaID = mediaID
	return e
}

// WithUser adds user context to the error
func (e *PlaybackError) WithUser(userID string) *PlaybackError {
	e.UserID = userID
	return e
}

// WithDevice adds device context to the error
func (e *PlaybackError) WithDevice(deviceID string) *PlaybackError {
	e.DeviceID = deviceID
	return e
}

// WithDetail adds a key-value detail to the error
func (e *PlaybackError) WithDetail(key string, value interface{}) *PlaybackError {
	e.Details[key] = value
	return e
}

// WithDetails adds multiple key-value details to the error
func (e *PlaybackError) WithDetails(details map[string]interface{}) *PlaybackError {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// IsRecoverable returns true if the error might succeed on retry
func (e *PlaybackError) IsRecoverable() bool {
	// Timeout errors are often recoverable
	if errors.Is(e.Err, ErrTimeout) {
		return true
	}
	
	// Stream interruptions might be temporary
	if errors.Is(e.Err, ErrStreamInterrupted) {
		return true
	}
	
	// Stream not ready might resolve
	if errors.Is(e.Err, ErrStreamNotReady) {
		return true
	}
	
	// Cleanup in progress might finish
	if errors.Is(e.Err, ErrCleanupInProgress) {
		return true
	}
	
	// Internal errors might be transient
	if e.Type == ErrorTypeInternal {
		return true
	}
	
	return false
}

// Error creation helpers

// DecisionError creates a playback decision error
func DecisionError(op string, err error) *PlaybackError {
	return New(ErrorTypeDecision, op, err)
}

// SessionError creates a session-related error
func SessionError(op string, err error) *PlaybackError {
	return New(ErrorTypeSession, op, err)
}

// StreamingError creates a streaming-related error
func StreamingError(op string, err error) *PlaybackError {
	return New(ErrorTypeStreaming, op, err)
}

// HistoryError creates a history tracking error
func HistoryError(op string, err error) *PlaybackError {
	return New(ErrorTypeHistory, op, err)
}

// CleanupError creates a cleanup operation error
func CleanupError(op string, err error) *PlaybackError {
	return New(ErrorTypeCleanup, op, err)
}

// DeviceError creates a device profile error
func DeviceError(op string, err error) *PlaybackError {
	return New(ErrorTypeDevice, op, err)
}

// ValidationError creates a validation error
func ValidationError(op string, err error) *PlaybackError {
	return New(ErrorTypeValidation, op, err)
}

// InternalError creates an internal system error
func InternalError(op string, err error) *PlaybackError {
	return New(ErrorTypeInternal, op, err)
}

// Wrap wraps an error with operation context if it's not already a PlaybackError
func Wrap(err error, errType ErrorType, op string) error {
	if err == nil {
		return nil
	}
	
	// If it's already a PlaybackError, preserve it
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return err
	}
	
	// Otherwise wrap it
	return New(errType, op, err)
}

// GetType extracts the error type from an error
func GetType(err error) ErrorType {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.Type
	}
	return ErrorTypeInternal
}

// GetOperation extracts the operation from an error
func GetOperation(err error) string {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.Op
	}
	return "unknown"
}

// GetSessionID extracts the session ID from an error
func GetSessionID(err error) string {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.SessionID
	}
	return ""
}

// GetMediaID extracts the media ID from an error
func GetMediaID(err error) string {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.MediaID
	}
	return ""
}

// GetUserID extracts the user ID from an error
func GetUserID(err error) string {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.UserID
	}
	return ""
}

// GetDeviceID extracts the device ID from an error
func GetDeviceID(err error) string {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.DeviceID
	}
	return ""
}

// GetDetails extracts error details
func GetDetails(err error) map[string]interface{} {
	var pErr *PlaybackError
	if errors.As(err, &pErr) {
		return pErr.Details
	}
	return nil
}