package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/images"
	domainimages "github.com/mantonx/viewra/internal/domain/images"
	infraimages "github.com/mantonx/viewra/internal/infrastructure/images"
)

// MockGetImageExecutor implements images.GetImageExecutor for testing
type MockGetImageExecutor struct {
	image *images.ImageResponse
	err   error
}

func (m *MockGetImageExecutor) Execute(ctx context.Context, id int) (*images.ImageResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.image, nil
}

// MockGetMediaImagesExecutor implements images.GetMediaImagesExecutor for testing
type MockGetMediaImagesExecutor struct {
	response *images.ListImagesResponse
	err      error
}

func (m *MockGetMediaImagesExecutor) Execute(ctx context.Context, mediaID int) (*images.ListImagesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// MockGetEntityImagesExecutor implements images.GetEntityImagesExecutor for testing
type MockGetEntityImagesExecutor struct {
	response *images.ListImagesResponse
	err      error
}

func (m *MockGetEntityImagesExecutor) Execute(ctx context.Context, mediaType domainimages.MediaType, entityID int) (*images.ListImagesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// MockGetBatchMediaImagesExecutor implements images.GetBatchMediaImagesExecutor for testing
type MockGetBatchMediaImagesExecutor struct {
	response *images.BatchImagesResponse
	err      error
}

func (m *MockGetBatchMediaImagesExecutor) Execute(ctx context.Context, mediaIDs []int, mediaType string, entityIDs []int) (*images.BatchImagesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestGetImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		imageID        string
		mockImage      *images.ImageResponse
		mockError      error
		expectedStatus int
		expectImage    bool
	}{
		{
			name:    "successful get image",
			imageID: "1",
			mockImage: &images.ImageResponse{
				ID:        1,
				MediaType: "movie",
				EntityID:  100,
				ImageType: "poster",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectImage:    true,
		},
		{
			name:           "invalid image ID",
			imageID:        "invalid",
			mockImage:      nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectImage:    false,
		},
		{
			name:           "image not found",
			imageID:        "999",
			mockImage:      nil,
			mockError:      errors.New("image not found"),
			expectedStatus: http.StatusNotFound,
			expectImage:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetImage := &MockGetImageExecutor{
				image: tt.mockImage,
				err:   tt.mockError,
			}

			handler := NewImagesHandler(
				mockGetImage,
				nil, // not needed for this test
				nil, // not needed for this test
				nil, // not needed for this test
				nil, // not needed for this test
				nil, // not needed for this test
			)

			req := httptest.NewRequest(http.MethodGet, "/api/images/"+tt.imageID, nil)
			w := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: tt.imageID}}

			handler.GetImage(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectImage {
				var response images.ImageResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if response.ID != tt.mockImage.ID {
					t.Errorf("Expected image ID %d, got %d", tt.mockImage.ID, response.ID)
				}
			}
		})
	}
}

func TestServeImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create temporary cache directory and file
	tempDir := t.TempDir()
	cacheService := infraimages.NewCacheService(tempDir)

	// Create a test preset file
	hash := "abcd1234567890"
	level1 := hash[0:2]
	level2 := hash[2:4]
	presetPath := filepath.Join(tempDir, level1, level2)
	os.MkdirAll(presetPath, 0755)
	testFile := filepath.Join(presetPath, hash+"_poster_medium.webp")
	os.WriteFile(testFile, []byte("fake image data"), 0644)

	tests := []struct {
		name           string
		imageID        string
		preset         string
		mockImage      *images.ImageResponse
		mockError      error
		expectedStatus int
		expectFile     bool
	}{
		{
			name:    "serve existing preset",
			imageID: "1",
			preset:  "medium",
			mockImage: &images.ImageResponse{
				ID:        1,
				ImageType: "poster",
				FileHash:  &hash,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectFile:     true,
		},
		{
			name:           "invalid image ID",
			imageID:        "invalid",
			preset:         "medium",
			mockImage:      nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:           "image not found",
			imageID:        "999",
			preset:         "medium",
			mockImage:      nil,
			mockError:      errors.New("image not found"),
			expectedStatus: http.StatusNotFound,
			expectFile:     false,
		},
		{
			name:    "invalid preset name",
			imageID: "1",
			preset:  "invalid",
			mockImage: &images.ImageResponse{
				ID:        1,
				ImageType: "poster",
				FileHash:  &hash,
			},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:    "missing file hash",
			imageID: "1",
			preset:  "medium",
			mockImage: &images.ImageResponse{
				ID:        1,
				ImageType: "poster",
				FileHash:  nil,
			},
			mockError:      nil,
			expectedStatus: http.StatusInternalServerError,
			expectFile:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetImage := &MockGetImageExecutor{
				image: tt.mockImage,
				err:   tt.mockError,
			}

			handler := NewImagesHandler(
				mockGetImage,
				nil,
				nil,
				nil,
				nil,
				cacheService,
			)

			req := httptest.NewRequest(http.MethodGet, "/api/images/"+tt.imageID+"/file?preset="+tt.preset, nil)
			w := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: tt.imageID}}

			handler.ServeImage(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectFile {
				contentType := w.Header().Get("Content-Type")
				if contentType != "image/webp" {
					t.Errorf("Expected Content-Type image/webp, got %s", contentType)
				}

				cacheControl := w.Header().Get("Cache-Control")
				if cacheControl != "public, max-age=31536000, immutable" {
					t.Errorf("Expected proper cache headers, got %s", cacheControl)
				}
			}
		})
	}
}

func TestGetMediaImages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		mockResponse   *images.ListImagesResponse
		mockError      error
		expectedStatus int
		expectImages   bool
	}{
		{
			name:    "successful get media images",
			mediaID: "100",
			mockResponse: &images.ListImagesResponse{
				Images: []images.ImageResponse{
					{ID: 1, MediaType: "movie", ImageType: "poster"},
					{ID: 2, MediaType: "movie", ImageType: "fanart"},
				},
				Total: 2,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectImages:   true,
		},
		{
			name:           "invalid media ID",
			mediaID:        "invalid",
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectImages:   false,
		},
		{
			name:    "no images found",
			mediaID: "100",
			mockResponse: &images.ListImagesResponse{
				Images: []images.ImageResponse{},
				Total:  0,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectImages:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetMediaImages := &MockGetMediaImagesExecutor{
				response: tt.mockResponse,
				err:      tt.mockError,
			}

			handler := NewImagesHandler(
				nil,
				mockGetMediaImages,
				nil,
				nil,
				nil,
				nil,
			)

			req := httptest.NewRequest(http.MethodGet, "/api/media/"+tt.mediaID+"/images", nil)
			w := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: tt.mediaID}}

			handler.GetMediaImages(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectImages {
				var response images.ListImagesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if response.Total != tt.mockResponse.Total {
					t.Errorf("Expected total %d, got %d", tt.mockResponse.Total, response.Total)
				}
			}
		})
	}
}

func TestGetTVShowImages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResponse := &images.ListImagesResponse{
		Images: []images.ImageResponse{
			{ID: 1, MediaType: "tv_show", ImageType: "poster"},
		},
		Total: 1,
	}

	mockGetEntityImages := &MockGetEntityImagesExecutor{
		response: mockResponse,
		err:      nil,
	}

	handler := NewImagesHandler(
		nil,
		nil,
		mockGetEntityImages,
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/tv/shows/1/images", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.GetTVShowImages(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response images.ListImagesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Total != 1 {
		t.Errorf("Expected total 1, got %d", response.Total)
	}
}

func TestGetBatchMediaImages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		mockResponse   *images.BatchImagesResponse
		mockError      error
		expectedStatus int
		expectBatch    bool
	}{
		{
			name: "successful batch get with media IDs",
			requestBody: map[string]interface{}{
				"media_ids": []int{1, 2, 3},
			},
			mockResponse: &images.BatchImagesResponse{
				MediaImages: map[int][]images.ImageResponse{
					1: {{ID: 1}},
					2: {{ID: 2}},
					3: {{ID: 3}},
				},
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectBatch:    true,
		},
		{
			name: "successful batch get with entity IDs",
			requestBody: map[string]interface{}{
				"entity_ids": []int{10, 20},
				"media_type": "tv_show",
			},
			mockResponse: &images.BatchImagesResponse{
				MediaImages: map[int][]images.ImageResponse{
					10: {{ID: 10}},
					20: {{ID: 20}},
				},
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectBatch:    true,
		},
		{
			name:           "missing both media_ids and entity_ids",
			requestBody:    map[string]interface{}{},
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectBatch:    false,
		},
		{
			name: "both media_ids and entity_ids provided",
			requestBody: map[string]interface{}{
				"media_ids":  []int{1},
				"entity_ids": []int{10},
				"media_type": "tv_show",
			},
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectBatch:    false,
		},
		{
			name: "batch size too large",
			requestBody: map[string]interface{}{
				"media_ids": make([]int, 201), // > 200 limit
			},
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectBatch:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetBatch := &MockGetBatchMediaImagesExecutor{
				response: tt.mockResponse,
				err:      tt.mockError,
			}

			handler := NewImagesHandler(
				nil,
				nil,
				nil,
				mockGetBatch,
				nil,
				nil,
			)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/images/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handler.GetBatchMediaImages(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectBatch {
				var response images.BatchImagesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(response.MediaImages) != len(tt.mockResponse.MediaImages) {
					t.Errorf("Expected %d items in batch, got %d", len(tt.mockResponse.MediaImages), len(response.MediaImages))
				}
			}
		})
	}
}
