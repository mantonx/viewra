// Package types provides common error types for proper error propagation
package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents standardized error codes across the application
type ErrorCode string

const (
	// General errors
	ErrorCodeUnknown    ErrorCode = "UNKNOWN_ERROR"
	ErrorCodeInternal   ErrorCode = "INTERNAL_ERROR"
	ErrorCodeValidation ErrorCode = "VALIDATION_ERROR"
	ErrorCodeNotFound   ErrorCode = "NOT_FOUND"
	ErrorCodeConflict   ErrorCode = "CONFLICT"
	ErrorCodeRateLimit  ErrorCode = "RATE_LIMIT"
	ErrorCodeTimeout    ErrorCode = "TIMEOUT"
	ErrorCodeCancelled  ErrorCode = "CANCELLED"

	// Media errors
	ErrorCodeMediaNotFound     ErrorCode = "MEDIA_NOT_FOUND"
	ErrorCodeMediaCorrupted    ErrorCode = "MEDIA_CORRUPTED"
	ErrorCodeMediaUnsupported  ErrorCode = "MEDIA_UNSUPPORTED"
	ErrorCodeMediaAccessDenied ErrorCode = "MEDIA_ACCESS_DENIED"
	ErrorCodeMediaInProcessing ErrorCode = "MEDIA_IN_PROCESSING"

	// Transcoding errors
	ErrorCodeTranscodingFailed      ErrorCode = "TRANSCODING_FAILED"
	ErrorCodeTranscodingUnavailable ErrorCode = "TRANSCODING_UNAVAILABLE"
	ErrorCodeTranscodingInProgress  ErrorCode = "TRANSCODING_IN_PROGRESS"
	ErrorCodeTranscodingCancelled   ErrorCode = "TRANSCODING_CANCELLED"
	ErrorCodeTranscodingTimeout     ErrorCode = "TRANSCODING_TIMEOUT"

	// Plugin errors
	ErrorCodePluginNotFound    ErrorCode = "PLUGIN_NOT_FOUND"
	ErrorCodePluginFailed      ErrorCode = "PLUGIN_FAILED"
	ErrorCodePluginTimeout     ErrorCode = "PLUGIN_TIMEOUT"
	ErrorCodePluginUnavailable ErrorCode = "PLUGIN_UNAVAILABLE"
	ErrorCodePluginConfig      ErrorCode = "PLUGIN_CONFIG_ERROR"

	// FFmpeg specific errors
	ErrorCodeFFmpegNotFound    ErrorCode = "FFMPEG_NOT_FOUND"
	ErrorCodeFFmpegFailed      ErrorCode = "FFMPEG_FAILED"
	ErrorCodeFFmpegKilled      ErrorCode = "FFMPEG_KILLED"
	ErrorCodeFFmpegInvalidArgs ErrorCode = "FFMPEG_INVALID_ARGS"
	ErrorCodeFFmpegUnsupported ErrorCode = "FFMPEG_UNSUPPORTED"

	// Resource errors
	ErrorCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"
	ErrorCodeDiskFull          ErrorCode = "DISK_FULL"
	ErrorCodeMemoryExhausted   ErrorCode = "MEMORY_EXHAUSTED"
	ErrorCodeCPUOverloaded     ErrorCode = "CPU_OVERLOADED"
	ErrorCodeGPUUnavailable    ErrorCode = "GPU_UNAVAILABLE"

	// Session errors
	ErrorCodeSessionNotFound     ErrorCode = "SESSION_NOT_FOUND"
	ErrorCodeSessionExpired      ErrorCode = "SESSION_EXPIRED"
	ErrorCodeSessionInvalid      ErrorCode = "SESSION_INVALID"
	ErrorCodeSessionLimitReached ErrorCode = "SESSION_LIMIT_REACHED"
)

// ErrorSeverity indicates the severity of an error
type ErrorSeverity string

const (
	SeverityInfo     ErrorSeverity = "info"
	SeverityWarning  ErrorSeverity = "warning"
	SeverityError    ErrorSeverity = "error"
	SeverityCritical ErrorSeverity = "critical"
)

// AppError represents a structured error with metadata
type AppError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Severity    ErrorSeverity          `json:"severity"`
	HTTPStatus  int                    `json:"http_status"`
	Context     map[string]interface{} `json:"context,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"` // User-friendly message
	Retryable   bool                   `json:"retryable"`
	RetryAfter  *time.Duration         `json:"retry_after,omitempty"`

	// Chain of errors for debugging
	Cause       error  `json:"-"`
	CauseString string `json:"cause,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithRequestID adds a request ID to the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithUserMessage sets a user-friendly error message
func (e *AppError) WithUserMessage(message string) *AppError {
	e.UserMessage = message
	return e
}

// WithRetryAfter marks the error as retryable after a specific duration
func (e *AppError) WithRetryAfter(duration time.Duration) *AppError {
	e.Retryable = true
	e.RetryAfter = &duration
	return e
}

// ToJSON converts the error to JSON
func (e *AppError) ToJSON() []byte {
	data, _ := json.Marshal(e)
	return data
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Severity:   SeverityError,
		HTTPStatus: httpStatus,
		Timestamp:  time.Now(),
		Retryable:  false,
	}
}

// NewAppErrorWithCause creates an error with an underlying cause
func NewAppErrorWithCause(code ErrorCode, message string, httpStatus int, cause error) *AppError {
	err := NewAppError(code, message, httpStatus)
	err.Cause = cause
	if cause != nil {
		err.CauseString = cause.Error()
	}
	return err
}

// Common error constructors

// NewValidationError creates a validation error
func NewValidationError(message string, details ...string) *AppError {
	err := NewAppError(ErrorCodeValidation, message, http.StatusBadRequest)
	if len(details) > 0 {
		err.Details = details[0]
	}
	err.Severity = SeverityWarning
	return err
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, id string) *AppError {
	return NewAppError(
		ErrorCodeNotFound,
		fmt.Sprintf("%s not found", resource),
		http.StatusNotFound,
	).WithContext("resource", resource).WithContext("id", id)
}

// NewInternalError creates an internal server error
func NewInternalError(message string, cause error) *AppError {
	err := NewAppErrorWithCause(ErrorCodeInternal, message, http.StatusInternalServerError, cause)
	err.Severity = SeverityCritical
	return err
}

// NewTranscodingError creates a transcoding-specific error
func NewTranscodingError(code ErrorCode, message string, cause error) *AppError {
	err := NewAppErrorWithCause(code, message, http.StatusInternalServerError, cause)
	err.Severity = SeverityError
	err.Retryable = isRetryableTranscodingError(code)
	return err
}

// NewPluginError creates a plugin-specific error
func NewPluginError(pluginName string, code ErrorCode, message string, cause error) *AppError {
	err := NewAppErrorWithCause(code, message, http.StatusServiceUnavailable, cause)
	err.WithContext("plugin", pluginName)
	err.Severity = SeverityError
	return err
}

// NewResourceError creates a resource-related error
func NewResourceError(code ErrorCode, message string) *AppError {
	err := NewAppError(code, message, http.StatusServiceUnavailable)
	err.Severity = SeverityCritical
	err.Retryable = true
	err.RetryAfter = &[]time.Duration{5 * time.Minute}[0]
	return err
}

// Helper functions

// isRetryableTranscodingError determines if a transcoding error is retryable
func isRetryableTranscodingError(code ErrorCode) bool {
	switch code {
	case ErrorCodeTranscodingTimeout,
		ErrorCodeTranscodingUnavailable,
		ErrorCodeFFmpegKilled:
		return true
	default:
		return false
	}
}

// HTTPStatusFromErrorCode maps error codes to HTTP status codes
func HTTPStatusFromErrorCode(code ErrorCode) int {
	switch code {
	case ErrorCodeValidation:
		return http.StatusBadRequest
	case ErrorCodeNotFound, ErrorCodeMediaNotFound, ErrorCodeSessionNotFound:
		return http.StatusNotFound
	case ErrorCodeConflict:
		return http.StatusConflict
	case ErrorCodeRateLimit:
		return http.StatusTooManyRequests
	case ErrorCodeTimeout, ErrorCodeTranscodingTimeout:
		return http.StatusGatewayTimeout
	case ErrorCodeCancelled, ErrorCodeTranscodingCancelled:
		return http.StatusRequestTimeout
	case ErrorCodeResourceExhausted, ErrorCodeDiskFull, ErrorCodeMemoryExhausted:
		return http.StatusServiceUnavailable
	case ErrorCodePluginNotFound, ErrorCodePluginUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Retryable
	}
	return false
}

// GetRetryAfter gets the retry-after duration from an error
func GetRetryAfter(err error) *time.Duration {
	if appErr, ok := err.(*AppError); ok {
		return appErr.RetryAfter
	}
	return nil
}
