package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/media"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
)

// Mock implementations for media use case interfaces

type mockGetMediaExecutor struct {
	executeFunc func(ctx context.Context, id int64) (media.MediaResponse, error)
}

func (m *mockGetMediaExecutor) Execute(ctx context.Context, id int64) (media.MediaResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, id)
	}
	return media.MediaResponse{}, nil
}

type mockListMediaExecutor struct {
	executeFunc    func(ctx context.Context, libraryID int64) (media.ListMediaResponse, error)
	executeAllFunc func(ctx context.Context) (media.ListMediaResponse, error)
}

func (m *mockListMediaExecutor) Execute(ctx context.Context, libraryID int64) (media.ListMediaResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, libraryID)
	}
	return media.ListMediaResponse{}, nil
}

func (m *mockListMediaExecutor) ExecuteAll(ctx context.Context) (media.ListMediaResponse, error) {
	if m.executeAllFunc != nil {
		return m.executeAllFunc(ctx)
	}
	return media.ListMediaResponse{}, nil
}

func TestMediaHandler_List(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		libraryID      string
		mockResponse   media.ListMediaResponse
		mockError      error
		expectedStatus int
	}{
		{
			name:      "successful list with library_id",
			libraryID: "1",
			mockResponse: media.ListMediaResponse{
				Media: []media.MediaResponse{
					{
						ID:        1,
						LibraryID: 1,
						Title:     "Test Movie 1",
						FilePath:  "/movies/movie1.mp4",
						FileSize:  1024000,
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        2,
						LibraryID: 1,
						Title:     "Test Movie 2",
						FilePath:  "/movies/movie2.mkv",
						FileSize:  2048000,
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
				Total: 2,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:      "missing library_id parameter - lists all media",
			libraryID: "",
			mockResponse: media.ListMediaResponse{
				Media: []media.MediaResponse{
					{
						ID:        1,
						LibraryID: 1,
						Title:     "Movie from Library 1",
						FilePath:  "/movies/movie1.mp4",
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        2,
						LibraryID: 2,
						Title:     "Movie from Library 2",
						FilePath:  "/other/movie2.mp4",
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
				Total: 2,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid library_id parameter",
			libraryID:      "invalid",
			mockResponse:   media.ListMediaResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty library (no media)",
			libraryID:      "1",
			mockResponse:   media.ListMediaResponse{Media: []media.MediaResponse{}, Total: 0},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "internal server error",
			libraryID:      "1",
			mockResponse:   media.ListMediaResponse{},
			mockError:      errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockList := &mockListMediaExecutor{
				executeFunc: func(_ context.Context, _ int64) (media.ListMediaResponse, error) {
					return tt.mockResponse, tt.mockError
				},
				executeAllFunc: func(_ context.Context) (media.ListMediaResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewMediaHandler(
				nil, // getMedia not used in this test
				mockList,
				nil, // streamInfo not used in this test
				nil, // getTracks not used in this test
			)

			c, w := setupTestContext(http.MethodGet, "/api/media?library_id="+tt.libraryID, nil)
			if tt.libraryID != "" {
				c.Request.URL.RawQuery = "library_id=" + tt.libraryID
			}
			handler.List(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var responseBody media.ListMediaResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.Total != tt.mockResponse.Total {
					t.Errorf("Expected total %d, got %d", tt.mockResponse.Total, responseBody.Total)
				}

				if len(responseBody.Media) != len(tt.mockResponse.Media) {
					t.Errorf("Expected %d media items, got %d", len(tt.mockResponse.Media), len(responseBody.Media))
				}
			}
		})
	}
}

func TestMediaHandler_Get(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		mediaID        string
		mockResponse   media.MediaResponse
		mockError      error
		expectedStatus int
	}{
		{
			name:    "successful get - movie",
			mediaID: "1",
			mockResponse: media.MediaResponse{
				ID:        1,
				LibraryID: 1,
				FilePath:  "/movies/movie1.mp4",
				FileSize:  1024000,
				Title:     "Test Movie",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:    "successful get - tv episode",
			mediaID: "2",
			mockResponse: media.MediaResponse{
				ID:        2,
				LibraryID: 2,
				FilePath:  "/tv/show/S01E01.mkv",
				FileSize:  2048000,
				Title:     "Pilot",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid media ID",
			mediaID:        "invalid",
			mockResponse:   media.MediaResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "media not found",
			mediaID:        "999",
			mockResponse:   media.MediaResponse{},
			mockError:      domainMedia.ErrMediaNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "internal server error",
			mediaID:        "1",
			mockResponse:   media.MediaResponse{},
			mockError:      errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGet := &mockGetMediaExecutor{
				executeFunc: func(_ context.Context, _ int64) (media.MediaResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewMediaHandler(
				mockGet,
				nil, // listMedia not used in this test
				nil, // streamInfo not used in this test
				nil, // getTracks not used in this test
			)

			c, w := setupTestContext(http.MethodGet, "/api/media/"+tt.mediaID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.mediaID}}
			handler.Get(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var responseBody media.GetMediaResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.Media.ID != tt.mockResponse.ID {
					t.Errorf("Expected media ID %d, got %d", tt.mockResponse.ID, responseBody.Media.ID)
				}

				if responseBody.Media.Title != tt.mockResponse.Title {
					t.Errorf("Expected title %s, got %s", tt.mockResponse.Title, responseBody.Media.Title)
				}
			}

			if tt.expectedStatus == http.StatusBadRequest {
				// Direct ErrorResponse from handler (validation error)
				var responseBody ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}

				if responseBody.Error != "Invalid media ID" {
					t.Errorf("Expected error 'Invalid media ID', got '%s'", responseBody.Error)
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
