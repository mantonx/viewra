package errors

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// ViewraError represents a structured error with HTTP context
type ViewraError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Cause      error                  `json:"-"`
	HTTPStatus int                    `json:"-"`
}

func (e *ViewraError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ViewraError) Unwrap() error {
	return e.Cause
}

// ToGinResponse sends the error as a standardized JSON response
func (e *ViewraError) ToGinResponse(c *gin.Context) {
	statusCode := e.HTTPStatus
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	response := gin.H{
		"error": e.Message,
		"code":  e.Code,
	}

	if e.Context != nil && len(e.Context) > 0 {
		response["details"] = e.Context
	}

	logger.Error("HTTP error response",
		"status", statusCode,
		"code", e.Code,
		"message", e.Message,
		"path", c.Request.URL.Path,
		"method", c.Request.Method)

	c.JSON(statusCode, response)
}

// Common error constructors
func NewValidationError(message string, field string) *ViewraError {
	return &ViewraError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
		Context:    map[string]interface{}{"field": field},
	}
}

func NewNotFoundError(resource string, id string) *ViewraError {
	return &ViewraError{
		Code:       "NOT_FOUND",
		Message:    resource + " not found",
		HTTPStatus: http.StatusNotFound,
		Context:    map[string]interface{}{"resource": resource, "id": id},
	}
}

func NewInternalError(message string, cause error) *ViewraError {
	return &ViewraError{
		Code:       "INTERNAL_ERROR",
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Cause:      cause,
	}
}

func NewDatabaseError(operation string, cause error) *ViewraError {
	return &ViewraError{
		Code:       "DATABASE_ERROR",
		Message:    "Database operation failed",
		HTTPStatus: http.StatusInternalServerError,
		Context:    map[string]interface{}{"operation": operation},
		Cause:      cause,
	}
}

func NewPluginError(pluginID string, operation string, cause error) *ViewraError {
	return &ViewraError{
		Code:       "PLUGIN_ERROR",
		Message:    "Plugin operation failed",
		HTTPStatus: http.StatusInternalServerError,
		Context:    map[string]interface{}{"plugin": pluginID, "operation": operation},
		Cause:      cause,
	}
}

// HTTP helpers to eliminate duplicate error handling

// HandleValidationError sends a validation error response
func HandleValidationError(c *gin.Context, message string, field string) {
	NewValidationError(message, field).ToGinResponse(c)
}

// HandleNotFound sends a not found error response
func HandleNotFound(c *gin.Context, resource string, id string) {
	NewNotFoundError(resource, id).ToGinResponse(c)
}

// HandleInternalError sends an internal server error response
func HandleInternalError(c *gin.Context, message string, err error) {
	NewInternalError(message, err).ToGinResponse(c)
}

// HandleDatabaseError sends a database error response
func HandleDatabaseError(c *gin.Context, operation string, err error) {
	NewDatabaseError(operation, err).ToGinResponse(c)
}

// UUID parsing helper
func ParseAndValidateUUID(c *gin.Context, paramName string) (string, bool) {
	id := c.Param(paramName)
	if id == "" {
		HandleValidationError(c, "Missing "+paramName, paramName)
		return "", false
	}

	// Basic UUID validation (you can make this more strict if needed)
	if len(id) < 32 {
		HandleValidationError(c, "Invalid "+paramName+" format", paramName)
		return "", false
	}

	return id, true
}
