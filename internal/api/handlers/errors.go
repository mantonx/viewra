package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	domainLibrary "github.com/viewra/viewra/internal/domain/library"
	domainMedia "github.com/viewra/viewra/internal/domain/media"
	domainScanner "github.com/viewra/viewra/internal/domain/scanner"
)

// ErrorResponse represents an API error response (legacy)
// Deprecated: Use APIError instead for new code
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// APIError represents a standardized error response structure
type APIError struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Details   any       `json:"details,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorMapper maps domain errors to HTTP responses
type ErrorMapper struct {
	logger *slog.Logger
}

// NewErrorMapper creates a new error mapper
func NewErrorMapper(logger *slog.Logger) *ErrorMapper {
	return &ErrorMapper{
		logger: logger,
	}
}

// MapError maps a domain error to HTTP status code and APIError
func (m *ErrorMapper) MapError(c *gin.Context, err error) (int, APIError) {
	requestID := getRequestID(c)
	timestamp := time.Now()

	apiErr := APIError{
		RequestID: requestID,
		Timestamp: timestamp,
	}

	// Map domain errors to API errors
	switch {
	// Library errors
	case errors.Is(err, domainLibrary.ErrLibraryNotFound):
		apiErr.Code = "LIBRARY_NOT_FOUND"
		apiErr.Message = "Library not found"
		return http.StatusNotFound, apiErr

	case errors.Is(err, domainLibrary.ErrDuplicatePath):
		apiErr.Code = "DUPLICATE_PATH"
		apiErr.Message = "A library with this path already exists"
		return http.StatusConflict, apiErr

	case errors.Is(err, domainLibrary.ErrPathNotFound):
		apiErr.Code = "PATH_NOT_FOUND"
		apiErr.Message = "The specified path does not exist on the filesystem"
		return http.StatusBadRequest, apiErr

	case errors.Is(err, domainLibrary.ErrPathNotAbsolute):
		apiErr.Code = "PATH_NOT_ABSOLUTE"
		apiErr.Message = "The path must be an absolute path"
		return http.StatusBadRequest, apiErr

	case errors.Is(err, domainLibrary.ErrInvalidType):
		apiErr.Code = "INVALID_LIBRARY_TYPE"
		apiErr.Message = "Invalid library type. Must be 'movies', 'tv', or 'music'"
		return http.StatusBadRequest, apiErr

	case errors.Is(err, domainLibrary.ErrInvalidName):
		apiErr.Code = "INVALID_LIBRARY_NAME"
		apiErr.Message = "Invalid library name"
		return http.StatusBadRequest, apiErr

	case errors.Is(err, domainLibrary.ErrInvalidPath):
		apiErr.Code = "INVALID_PATH"
		apiErr.Message = "Invalid library path"
		return http.StatusBadRequest, apiErr

	// Media errors
	case errors.Is(err, domainMedia.ErrMediaNotFound):
		apiErr.Code = "MEDIA_NOT_FOUND"
		apiErr.Message = "Media item not found"
		return http.StatusNotFound, apiErr

	case errors.Is(err, domainMedia.ErrInvalidLibraryID):
		apiErr.Code = "INVALID_LIBRARY_ID"
		apiErr.Message = "Invalid library ID"
		return http.StatusBadRequest, apiErr

	case errors.Is(err, domainMedia.ErrDuplicateFilePath):
		apiErr.Code = "DUPLICATE_FILE_PATH"
		apiErr.Message = "Media file already exists in library"
		return http.StatusConflict, apiErr

	// Scanner errors
	case errors.Is(err, domainScanner.ErrAlreadyRunning):
		apiErr.Code = "SCAN_ALREADY_RUNNING"
		apiErr.Message = "A scan is already running for this library"
		return http.StatusConflict, apiErr

	case errors.Is(err, domainScanner.ErrNotFound):
		apiErr.Code = "SCAN_JOB_NOT_FOUND"
		apiErr.Message = "Scan job not found"
		return http.StatusNotFound, apiErr

	case errors.Is(err, domainScanner.ErrNotRunning):
		apiErr.Code = "SCAN_NOT_RUNNING"
		apiErr.Message = "Scan is not running"
		return http.StatusBadRequest, apiErr

	// Unknown error - log it and return generic error
	default:
		m.logger.Error("unmapped error",
			"error", err,
			"request_id", requestID,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)

		apiErr.Code = "INTERNAL_ERROR"
		apiErr.Message = "An internal error occurred"
		return http.StatusInternalServerError, apiErr
	}
}

// RespondWithError sends a standardized error response
func (m *ErrorMapper) RespondWithError(c *gin.Context, err error) {
	status, apiErr := m.MapError(c, err)
	c.JSON(status, apiErr)
}

// getRequestID retrieves the request ID from the context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// handleError converts domain errors to appropriate HTTP responses
func handleError(c *gin.Context, err error) {
	switch {
	// Library errors
	case errors.Is(err, domainLibrary.ErrLibraryNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Library not found",
			Message: err.Error(),
		})

	case errors.Is(err, domainLibrary.ErrInvalidPath),
		errors.Is(err, domainLibrary.ErrEmptyPath),
		errors.Is(err, domainLibrary.ErrPathNotAbsolute),
		errors.Is(err, domainLibrary.ErrPathTraversal):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library path",
			Message: err.Error(),
		})

	case errors.Is(err, domainLibrary.ErrPathNotFound),
		errors.Is(err, domainLibrary.ErrPathNotAccessible),
		errors.Is(err, domainLibrary.ErrPathNotReadable),
		errors.Is(err, domainLibrary.ErrPathNotDirectory):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Library path does not exist or is not accessible",
			Message: err.Error(),
		})

	case errors.Is(err, domainLibrary.ErrDuplicatePath):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "Library path already exists",
			Message: err.Error(),
		})

	case errors.Is(err, domainLibrary.ErrInvalidName),
		errors.Is(err, domainLibrary.ErrNameTooLong):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library name",
			Message: err.Error(),
		})

	case errors.Is(err, domainLibrary.ErrInvalidType):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library type",
			Message: err.Error(),
		})

	// Media errors
	case errors.Is(err, domainMedia.ErrMediaNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Media not found",
			Message: err.Error(),
		})

	case errors.Is(err, domainMedia.ErrInvalidLibraryID):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library ID",
			Message: err.Error(),
		})

	case errors.Is(err, domainMedia.ErrEmptyFilePath),
		errors.Is(err, domainMedia.ErrAbsoluteFilePath),
		errors.Is(err, domainMedia.ErrPathTraversal),
		errors.Is(err, domainMedia.ErrMissingFileExtension):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media file path",
			Message: err.Error(),
		})

	case errors.Is(err, domainMedia.ErrDuplicateFilePath):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "Media file already exists",
			Message: err.Error(),
		})

	// Scanner errors
	case errors.Is(err, domainScanner.ErrNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Scan job not found",
			Message: err.Error(),
		})

	case errors.Is(err, domainScanner.ErrAlreadyRunning):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "Scan already in progress",
			Message: err.Error(),
		})

	case errors.Is(err, domainScanner.ErrNotRunning):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Scan is not running",
			Message: err.Error(),
		})

	case errors.Is(err, domainScanner.ErrInvalidPath),
		errors.Is(err, domainScanner.ErrPathNotExist),
		errors.Is(err, domainScanner.ErrPathNotDirectory):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid scan path",
			Message: err.Error(),
		})

	case errors.Is(err, domainScanner.ErrInvalidStatus):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid scan status",
			Message: err.Error(),
		})

	// Default to internal server error
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal server error",
			Message: err.Error(),
		})
	}
}
