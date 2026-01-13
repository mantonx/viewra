package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	domainLibrary "github.com/mantonx/viewra/internal/domain/library"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
	domainScanner "github.com/mantonx/viewra/internal/domain/scanner"
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

		// Scanner errors
		{
			name:           "scan job not found",
			err:            domainScanner.ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "Scan job not found",
		},
		{
			name:           "scan already running",
			err:            domainScanner.ErrAlreadyRunning,
			expectedStatus: http.StatusConflict,
			expectedError:  "A scan is already running for this library",
		},
		{
			name:           "scan not running",
			err:            domainScanner.ErrNotRunning,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Scan is not running",
		},
		{
			name:           "invalid scan path",
			err:            domainScanner.ErrInvalidPath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid scan path",
		},
		{
			name:           "scan path not exist",
			err:            domainScanner.ErrPathNotExist,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid scan path",
		},
		{
			name:           "scan path not directory",
			err:            domainScanner.ErrPathNotDirectory,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid scan path",
		},
		{
			name:           "invalid scan status",
			err:            domainScanner.ErrInvalidStatus,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid scan status",
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

			// Parse the JSON response and validate APIError structure
			var apiErr APIError
			if err := json.Unmarshal(w.Body.Bytes(), &apiErr); err != nil {
				t.Errorf("Failed to parse APIError response: %v", err)
				return
			}

			// Verify APIError has required fields
			if apiErr.Code == "" {
				t.Error("Expected APIError.Code to be set")
			}
			if apiErr.Message == "" {
				t.Error("Expected APIError.Message to be set")
			}
			if apiErr.Timestamp.IsZero() {
				t.Error("Expected APIError.Timestamp to be set")
			}
		})
	}
}

func TestGetRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with request_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("request_id", "test-request-123")

		got := getRequestID(c)
		if got != "test-request-123" {
			t.Errorf("getRequestID() = %q, want %q", got, "test-request-123")
		}
	})

	t.Run("without request_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		got := getRequestID(c)
		if got != "" {
			t.Errorf("getRequestID() = %q, want empty string", got)
		}
	})

	t.Run("with non-string request_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("request_id", 12345) // int, not string

		got := getRequestID(c)
		if got != "" {
			t.Errorf("getRequestID() = %q, want empty string for non-string", got)
		}
	})
}

func TestRespondError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sends correct status and APIError", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("request_id", "req-456")

		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The input is invalid")

		if w.Code != http.StatusBadRequest {
			t.Errorf("respondError() status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var apiErr APIError
		if err := json.Unmarshal(w.Body.Bytes(), &apiErr); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if apiErr.Code != "INVALID_INPUT" {
			t.Errorf("apiErr.Code = %q, want %q", apiErr.Code, "INVALID_INPUT")
		}
		if apiErr.Message != "The input is invalid" {
			t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "The input is invalid")
		}
		if apiErr.RequestID != "req-456" {
			t.Errorf("apiErr.RequestID = %q, want %q", apiErr.RequestID, "req-456")
		}
		if apiErr.Timestamp.IsZero() {
			t.Error("apiErr.Timestamp should be set")
		}
	})
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
