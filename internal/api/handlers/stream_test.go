package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/media"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/streaming"
)

func TestStreamHandler_Stream(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	testContent := []byte("This is test video content for streaming")
	if err := os.WriteFile(testFile, testContent, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	now := time.Now()

	tests := []struct {
		name           string
		mediaID        string
		rangeHeader    string
		mockResponse   media.MediaResponse
		mockError      error
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:        "successful full file stream",
			mediaID:     "1",
			rangeHeader: "",
			mockResponse: media.MediaResponse{
				ID:        1,
				LibraryID: 1,
				Title:     "Test Video",
				FilePath:  testFile,
				FileSize:  int64(len(testContent)),
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:        "successful partial content stream",
			mediaID:     "1",
			rangeHeader: "bytes=0-10",
			mockResponse: media.MediaResponse{
				ID:        1,
				LibraryID: 1,
				Title:     "Test Video",
				FilePath:  testFile,
				FileSize:  int64(len(testContent)),
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusPartialContent,
			checkHeaders:   true,
		},
		{
			name:           "invalid media ID",
			mediaID:        "invalid",
			rangeHeader:    "",
			mockResponse:   media.MediaResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			checkHeaders:   false,
		},
		{
			name:           "media not found",
			mediaID:        "999",
			rangeHeader:    "",
			mockResponse:   media.MediaResponse{},
			mockError:      domainMedia.ErrMediaNotFound,
			expectedStatus: http.StatusNotFound,
			checkHeaders:   false,
		},
		{
			name:        "file not found (invalid file path)",
			mediaID:     "1",
			rangeHeader: "",
			mockResponse: media.MediaResponse{
				ID:        1,
				LibraryID: 1,
				Title:     "Test Video",
				FilePath:  "/nonexistent/file.mp4",
				FileSize:  1024000,
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError: nil,
			// When PrepareStream fails and stream is nil, handler returns 416
			expectedStatus: http.StatusRequestedRangeNotSatisfiable,
			checkHeaders:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGet := &mockGetMediaExecutor{
				executeFunc: func(_ context.Context, _ int64) (media.MediaResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			streamService := streaming.NewService()
			handler := NewStreamHandler(mockGet, streamService, nil)

			c, w := setupTestContext(http.MethodGet, "/api/stream/"+tt.mediaID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.mediaID}}
			if tt.rangeHeader != "" {
				c.Request.Header.Set("Range", tt.rangeHeader)
			}
			handler.Stream(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkHeaders {
				// Check Accept-Ranges header
				if acceptRanges := w.Header().Get("Accept-Ranges"); acceptRanges != "bytes" {
					t.Errorf("Expected Accept-Ranges: bytes, got: %s", acceptRanges)
				}

				// Check Content-Type header
				if contentType := w.Header().Get("Content-Type"); contentType == "" {
					t.Errorf("Expected Content-Type header to be set")
				}

				// Check Content-Length header
				if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
					t.Errorf("Expected Content-Length header to be set")
				}

				// Check Content-Range header for partial content
				if tt.expectedStatus == http.StatusPartialContent {
					if contentRange := w.Header().Get("Content-Range"); contentRange == "" {
						t.Errorf("Expected Content-Range header for partial content")
					}
				}

				// Verify some content was written
				if w.Body.Len() == 0 {
					t.Errorf("Expected response body to contain content")
				}
			}

			if tt.expectedStatus == http.StatusBadRequest {
				var responseBody APIError
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}

				if responseBody.Code != "INVALID_MEDIA_ID" {
					t.Errorf("Expected code 'INVALID_MEDIA_ID', got '%s'", responseBody.Code)
				}
			}

			if tt.expectedStatus == http.StatusNotFound {
				// APIError from handleError (domain error)
				var responseBody APIError
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}

				if responseBody.Code != "MEDIA_NOT_FOUND" {
					t.Errorf("Expected code 'MEDIA_NOT_FOUND', got '%s'", responseBody.Code)
				}
			}
		})
	}
}

func TestStreamHandler_Stream_InvalidRange(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	testContent := []byte("Small content")
	if err := os.WriteFile(testFile, testContent, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	now := time.Now()

	mockGet := &mockGetMediaExecutor{
		executeFunc: func(_ context.Context, _ int64) (media.MediaResponse, error) {
			return media.MediaResponse{
				ID:        1,
				LibraryID: 1,
				Title:     "Test Video",
				FilePath:  testFile,
				FileSize:  int64(len(testContent)),
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}

	streamService := streaming.NewService()
	handler := NewStreamHandler(mockGet, streamService, nil)

	// Test with invalid range (beyond file size)
	c, w := setupTestContext(http.MethodGet, "/api/stream/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request.Header.Set("Range", "bytes=1000-2000")
	handler.Stream(c)

	// Should return 416 Range Not Satisfiable
	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected status %d for invalid range, got %d", http.StatusRequestedRangeNotSatisfiable, w.Code)
	}

	// Check Content-Range header for error
	if contentRange := w.Header().Get("Content-Range"); contentRange != "bytes */*" {
		t.Errorf("Expected Content-Range: bytes */* for invalid range, got: %s", contentRange)
	}

	var responseBody APIError
	if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if responseBody.Code != "INVALID_RANGE" {
		t.Errorf("Expected code 'INVALID_RANGE', got '%s'", responseBody.Code)
	}
}

func TestStreamHandler_Stream_InternalError(t *testing.T) {
	mockGet := &mockGetMediaExecutor{
		executeFunc: func(_ context.Context, _ int64) (media.MediaResponse, error) {
			return media.MediaResponse{}, errors.New("database error")
		},
	}

	streamService := streaming.NewService()
	handler := NewStreamHandler(mockGet, streamService, nil)

	c, w := setupTestContext(http.MethodGet, "/api/stream/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Stream(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// APIError from handleError (unknown error goes to default case)
	var responseBody APIError
	if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if responseBody.Code != "INTERNAL_ERROR" {
		t.Errorf("Expected code 'INTERNAL_ERROR', got '%s'", responseBody.Code)
	}
}
