package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	domainLibrary "github.com/viewra/viewra/internal/domain/library"
	domainMedia "github.com/viewra/viewra/internal/domain/media"
	domainScanner "github.com/viewra/viewra/internal/domain/scanner"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
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
