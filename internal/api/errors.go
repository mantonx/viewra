// Package api provides error handling utilities for HTTP APIs
package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/types"
)

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Error   ErrorDetails `json:"error"`
	Success bool         `json:"success"`
}

// ErrorDetails contains detailed error information
type ErrorDetails struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"`
	Retryable   bool                   `json:"retryable"`
	RetryAfter  int                    `json:"retry_after,omitempty"` // seconds
	Context     map[string]interface{} `json:"context,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
}

// RespondWithError sends a structured error response
func RespondWithError(c *gin.Context, err error) {
	// Extract request ID if available
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = c.GetHeader("X-Request-ID")
	}

	// Check if it's an AppError
	var appErr *types.AppError
	if errors.As(err, &appErr) {
		// Use the structured error information
		response := ErrorResponse{
			Success: false,
			Error: ErrorDetails{
				Code:        string(appErr.Code),
				Message:     appErr.Message,
				Details:     appErr.Details,
				UserMessage: appErr.UserMessage,
				Retryable:   appErr.Retryable,
				Context:     appErr.Context,
				RequestID:   requestID,
			},
		}

		// Add retry-after if specified
		if appErr.RetryAfter != nil {
			response.Error.RetryAfter = int(appErr.RetryAfter.Seconds())
			c.Header("Retry-After", strings.TrimSpace(appErr.RetryAfter.String()))
		}

		// Log the error with appropriate severity
		logError(appErr, requestID)

		// Send response
		c.JSON(appErr.HTTPStatus, response)
		return
	}

	// Handle generic errors
	httpStatus := http.StatusInternalServerError
	errorCode := types.ErrorCodeInternal

	// Try to determine error type from error message
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		httpStatus = http.StatusNotFound
		errorCode = types.ErrorCodeNotFound
	case strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "required"):
		httpStatus = http.StatusBadRequest
		errorCode = types.ErrorCodeValidation
	case strings.Contains(errMsg, "timeout"):
		httpStatus = http.StatusGatewayTimeout
		errorCode = types.ErrorCodeTimeout
	case strings.Contains(errMsg, "cancelled") || strings.Contains(errMsg, "canceled"):
		httpStatus = http.StatusRequestTimeout
		errorCode = types.ErrorCodeCancelled
	}

	// Create response for generic error
	response := ErrorResponse{
		Success: false,
		Error: ErrorDetails{
			Code:      string(errorCode),
			Message:   errMsg,
			RequestID: requestID,
		},
	}

	// Log the error
	logger.Error("unstructured error", "error", err, "request_id", requestID)

	// Send response
	c.JSON(httpStatus, response)
}

// RespondWithAppError sends a structured AppError response
func RespondWithAppError(c *gin.Context, code types.ErrorCode, message string, httpStatus int) {
	appErr := types.NewAppError(code, message, httpStatus)
	RespondWithError(c, appErr)
}

// RespondWithValidationError sends a validation error response
func RespondWithValidationError(c *gin.Context, message string, details ...string) {
	appErr := types.NewValidationError(message, details...)
	RespondWithError(c, appErr)
}

// RespondWithNotFound sends a not found error response
func RespondWithNotFound(c *gin.Context, resource string, id string) {
	appErr := types.NewNotFoundError(resource, id)
	RespondWithError(c, appErr)
}

// RespondWithInternalError sends an internal error response
func RespondWithInternalError(c *gin.Context, message string, cause error) {
	appErr := types.NewInternalError(message, cause)
	RespondWithError(c, appErr)
}

// logError logs the error with appropriate severity
func logError(err *types.AppError, requestID string) {
	fields := []interface{}{
		"error_code", err.Code,
		"error_message", err.Message,
		"request_id", requestID,
	}

	if err.Details != "" {
		fields = append(fields, "details", err.Details)
	}

	if err.Context != nil {
		for k, v := range err.Context {
			fields = append(fields, k, v)
		}
	}

	if err.Cause != nil {
		fields = append(fields, "cause", err.Cause.Error())
	}

	switch err.Severity {
	case types.SeverityCritical:
		logger.Error("critical error", fields...)
	case types.SeverityError:
		logger.Error("error occurred", fields...)
	case types.SeverityWarning:
		logger.Warn("warning", fields...)
	case types.SeverityInfo:
		logger.Info("info", fields...)
	default:
		logger.Error("error occurred", fields...)
	}
}

// ErrorMiddleware is a middleware that recovers from panics and handles errors
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Convert panic to error
				var err error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = errors.New(v)
				default:
					err = errors.New("unknown panic")
				}

				// Create a critical error
				appErr := types.NewInternalError("panic recovered", err)
				appErr.Severity = types.SeverityCritical

				// Log stack trace
				logger.Error("panic recovered",
					"error", err,
					"request_path", c.Request.URL.Path,
					"request_method", c.Request.Method,
				)

				// Respond with error
				RespondWithError(c, appErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}
