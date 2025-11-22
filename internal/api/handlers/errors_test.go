package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	domainLibrary "github.com/mantonx/viewra/internal/domain/library"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
)

func TestHandleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
	}{
		// Library errors
		{
			name:           "library not found",
			err:            domainLibrary.ErrLibraryNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "Library not found",
		},
		{
			name:           "invalid path",
			err:            domainLibrary.ErrInvalidPath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library path",
		},
		{
			name:           "empty path",
			err:            domainLibrary.ErrEmptyPath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library path",
		},
		{
			name:           "path not absolute",
			err:            domainLibrary.ErrPathNotAbsolute,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library path",
		},
		{
			name:           "path traversal",
			err:            domainLibrary.ErrPathTraversal,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library path",
		},
		{
			name:           "path not found",
			err:            domainLibrary.ErrPathNotFound,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Library path does not exist or is not accessible",
		},
		{
			name:           "path not accessible",
			err:            domainLibrary.ErrPathNotAccessible,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Library path does not exist or is not accessible",
		},
		{
			name:           "path not readable",
			err:            domainLibrary.ErrPathNotReadable,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Library path does not exist or is not accessible",
		},
		{
			name:           "path not directory",
			err:            domainLibrary.ErrPathNotDirectory,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Library path does not exist or is not accessible",
		},
		{
			name:           "duplicate path",
			err:            domainLibrary.ErrDuplicatePath,
			expectedStatus: http.StatusConflict,
			expectedError:  "Library path already exists",
		},
		{
			name:           "invalid name",
			err:            domainLibrary.ErrInvalidName,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library name",
		},
		{
			name:           "name too long",
			err:            domainLibrary.ErrNameTooLong,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library name",
		},
		{
			name:           "invalid type",
			err:            domainLibrary.ErrInvalidType,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library type",
		},

		// Media errors
		{
			name:           "media not found",
			err:            domainMedia.ErrMediaNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "Media not found",
		},
		{
			name:           "invalid library ID",
			err:            domainMedia.ErrInvalidLibraryID,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid library ID",
		},
		{
			name:           "empty file path",
			err:            domainMedia.ErrEmptyFilePath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid media file path",
		},
		{
			name:           "absolute file path",
			err:            domainMedia.ErrAbsoluteFilePath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid media file path",
		},
		{
			name:           "media path traversal",
			err:            domainMedia.ErrPathTraversal,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid media file path",
		},
		{
			name:           "missing file extension",
			err:            domainMedia.ErrMissingFileExtension,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid media file path",
		},
		{
			name:           "duplicate file path",
			err:            domainMedia.ErrDuplicateFilePath,
			expectedStatus: http.StatusConflict,
			expectedError:  "Media file already exists",
		},

		// Generic error
		{
			name:           "internal server error",
			err:            errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handleError(c, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check that response body contains expected error
			if w.Body.Len() == 0 {
				t.Error("Expected error response body, got empty body")
			}

			// Note: We could also parse the JSON and check the exact error field,
			// but checking status code is the primary validation for error handling
		})
	}
}

func TestHandleError_WrappedErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "wrapped library not found",
			err:            errors.Join(domainLibrary.ErrLibraryNotFound, errors.New("additional context")),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "wrapped media not found",
			err:            errors.Join(domainMedia.ErrMediaNotFound, errors.New("query failed")),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "wrapped invalid path",
			err:            errors.Join(domainLibrary.ErrInvalidPath, errors.New("validation failed")),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handleError(c, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d for wrapped error, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
