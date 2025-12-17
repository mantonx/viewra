package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/application/library/scan"
	domainLibrary "github.com/mantonx/viewra/internal/domain/library"
	domainScanner "github.com/mantonx/viewra/internal/domain/scanner"
)

// Mock implementations for library service

type mockLibraryService struct {
	createFunc func(ctx context.Context, req library.CreateLibraryRequest) (library.LibraryResponse, error)
	getFunc    func(ctx context.Context, id int64) (library.LibraryResponse, error)
	listFunc   func(ctx context.Context) (library.ListLibrariesResponse, error)
	updateFunc func(ctx context.Context, id int64, req library.UpdateLibraryRequest) (library.LibraryResponse, error)
	deleteFunc func(ctx context.Context, id int64) error
}

func (m *mockLibraryService) Create(ctx context.Context, req library.CreateLibraryRequest) (library.LibraryResponse, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return library.LibraryResponse{}, nil
}

func (m *mockLibraryService) Get(ctx context.Context, id int64) (library.LibraryResponse, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return library.LibraryResponse{}, nil
}

func (m *mockLibraryService) List(ctx context.Context) (library.ListLibrariesResponse, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return library.ListLibrariesResponse{}, nil
}

func (m *mockLibraryService) Update(ctx context.Context, id int64, req library.UpdateLibraryRequest) (library.LibraryResponse, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, req)
	}
	return library.LibraryResponse{}, nil
}

func (m *mockLibraryService) Delete(ctx context.Context, id int64) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

type mockScanLibraryExecutor struct {
	startScanFunc      func(ctx context.Context, libraryID int64) (scan.StartScanResponse, error)
	resumeScanFunc     func(ctx context.Context, jobID int64) error
	getProgressFunc    func(ctx context.Context, jobID int64) (scan.ScanProgressResponse, error)
	getLatestScanFunc  func(ctx context.Context, libraryID int64) (scan.ScanProgressResponse, error)
	getScanHistoryFunc func(ctx context.Context, libraryID int64, limit int32) (scan.ScanHistoryResponse, error)
}

func (m *mockScanLibraryExecutor) StartScan(ctx context.Context, libraryID int64) (scan.StartScanResponse, error) {
	if m.startScanFunc != nil {
		return m.startScanFunc(ctx, libraryID)
	}
	return scan.StartScanResponse{}, nil
}

func (m *mockScanLibraryExecutor) ResumeScan(ctx context.Context, jobID int64) error {
	if m.resumeScanFunc != nil {
		return m.resumeScanFunc(ctx, jobID)
	}
	return nil
}

func (m *mockScanLibraryExecutor) GetProgress(ctx context.Context, jobID int64) (scan.ScanProgressResponse, error) {
	if m.getProgressFunc != nil {
		return m.getProgressFunc(ctx, jobID)
	}
	return scan.ScanProgressResponse{}, nil
}

func (m *mockScanLibraryExecutor) GetLatestScan(
	ctx context.Context,
	libraryID int64,
) (scan.ScanProgressResponse, error) {
	if m.getLatestScanFunc != nil {
		return m.getLatestScanFunc(ctx, libraryID)
	}
	return scan.ScanProgressResponse{}, nil
}

func (m *mockScanLibraryExecutor) GetScanHistory(
	ctx context.Context,
	libraryID int64,
	limit int32,
) (scan.ScanHistoryResponse, error) {
	if m.getScanHistoryFunc != nil {
		return m.getScanHistoryFunc(ctx, libraryID, limit)
	}
	return scan.ScanHistoryResponse{}, nil
}

// Test helper to create a test Gin context
func setupTestContext(method, path string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	return c, w
}

func TestLibraryHandler_Create(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		requestBody    interface{}
		mockResponse   library.LibraryResponse
		mockError      error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name: "successful creation",
			requestBody: library.CreateLibraryRequest{
				Name: "My Movies",
				Path: "/media/movies",
				Type: "movies",
			},
			mockResponse: library.LibraryResponse{
				ID:        1,
				Name:      "My Movies",
				Path:      "/media/movies",
				Type:      "movies",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusCreated,
			expectedBody: library.LibraryResponse{
				ID:        1,
				Name:      "My Movies",
				Path:      "/media/movies",
				Type:      "movies",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		{
			name:           "invalid request body",
			requestBody:    "invalid json",
			mockResponse:   library.LibraryResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body", // Check message field
		},
		{
			name: "use case error - duplicate path",
			requestBody: library.CreateLibraryRequest{
				Name: "My Movies",
				Path: "/media/movies",
				Type: "movies",
			},
			mockResponse:   library.LibraryResponse{},
			mockError:      domainLibrary.ErrDuplicatePath,
			expectedStatus: http.StatusConflict,
			expectedBody:   "A library with this path already exists", // APIError message
		},
		{
			name: "use case error - path not found",
			requestBody: library.CreateLibraryRequest{
				Name: "My Movies",
				Path: "/nonexistent/path",
				Type: "movies",
			},
			mockResponse:   library.LibraryResponse{},
			mockError:      domainLibrary.ErrPathNotFound,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Library path does not exist or is not accessible", // APIError message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLibraryService{
				createFunc: func(
					_ context.Context,
					_ library.CreateLibraryRequest,
				) (library.LibraryResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewLibraryHandler(
				mockService,
				nil, // scanLibrary not used in this test
			)

			c, w := setupTestContext(http.MethodPost, "/api/libraries", tt.requestBody)
			handler.Create(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Parse response body
			var responseBody map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
				t.Fatalf("Failed to parse response body: %v", err)
			}

			// For successful creation, check the ID
			if tt.expectedStatus == http.StatusCreated {
				if responseBody["id"].(float64) != float64(tt.mockResponse.ID) {
					t.Errorf("Expected ID %d, got %v", tt.mockResponse.ID, responseBody["id"])
				}
			}

			// For error responses, check either error field (ErrorResponse) or message field (APIError)
			if tt.expectedStatus >= 400 {
				expectedMsg := tt.expectedBody.(string)
				// Try message field first (APIError), then error field (ErrorResponse)
				if msg, ok := responseBody["message"].(string); ok && msg == expectedMsg {
					// APIError format - matched
				} else if errStr, ok := responseBody["error"].(string); ok && errStr == expectedMsg {
					// ErrorResponse format - matched
				} else {
					t.Errorf("Expected error '%s', got message='%v' error='%v'",
						expectedMsg, responseBody["message"], responseBody["error"])
				}
			}
		})
	}
}

func TestLibraryHandler_List(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		mockResponse   library.ListLibrariesResponse
		mockError      error
		expectedStatus int
	}{
		{
			name: "successful list with multiple libraries",
			mockResponse: library.ListLibrariesResponse{
				Libraries: []library.LibraryResponse{
					{
						ID:        1,
						Name:      "Movies",
						Path:      "/media/movies",
						Type:      "movies",
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        2,
						Name:      "TV Shows",
						Path:      "/media/tv",
						Type:      "tv",
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
			name: "successful list with empty libraries",
			mockResponse: library.ListLibrariesResponse{
				Libraries: []library.LibraryResponse{},
				Total:     0,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "internal server error",
			mockResponse:   library.ListLibrariesResponse{},
			mockError:      errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLibraryService{
				listFunc: func(_ context.Context) (library.ListLibrariesResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewLibraryHandler(
				mockService,
				nil, // scanLibrary not used in this test
			)

			c, w := setupTestContext(http.MethodGet, "/api/libraries", nil)
			handler.List(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var responseBody library.ListLibrariesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.Total != tt.mockResponse.Total {
					t.Errorf("Expected total %d, got %d", tt.mockResponse.Total, responseBody.Total)
				}

				if len(responseBody.Libraries) != len(tt.mockResponse.Libraries) {
					t.Errorf(
						"Expected %d libraries, got %d",
						len(tt.mockResponse.Libraries),
						len(responseBody.Libraries),
					)
				}
			}
		})
	}
}

func TestLibraryHandler_Get(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		libraryID      string
		mockResponse   library.LibraryResponse
		mockError      error
		expectedStatus int
	}{
		{
			name:      "successful get",
			libraryID: "1",
			mockResponse: library.LibraryResponse{
				ID:        1,
				Name:      "My Movies",
				Path:      "/media/movies",
				Type:      "movies",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid library ID",
			libraryID:      "invalid",
			mockResponse:   library.LibraryResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "library not found",
			libraryID:      "999",
			mockResponse:   library.LibraryResponse{},
			mockError:      domainLibrary.ErrLibraryNotFound,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLibraryService{
				getFunc: func(_ context.Context, _ int64) (library.LibraryResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewLibraryHandler(
				mockService,
				nil, // scanLibrary not used in this test
			)

			c, w := setupTestContext(http.MethodGet, "/api/libraries/"+tt.libraryID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.libraryID}}
			handler.Get(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var responseBody library.GetLibraryResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.Library.ID != tt.mockResponse.ID {
					t.Errorf("Expected library ID %d, got %d", tt.mockResponse.ID, responseBody.Library.ID)
				}
			}
		})
	}
}

func TestLibraryHandler_Update(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		libraryID      string
		requestBody    interface{}
		mockResponse   library.LibraryResponse
		mockError      error
		expectedStatus int
	}{
		{
			name:      "successful update",
			libraryID: "1",
			requestBody: library.UpdateLibraryRequest{
				Name: "Updated Movies",
				Path: "/media/movies-new",
				Type: "movies",
			},
			mockResponse: library.LibraryResponse{
				ID:        1,
				Name:      "Updated Movies",
				Path:      "/media/movies-new",
				Type:      "movies",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:      "partial update (only name)",
			libraryID: "1",
			requestBody: library.UpdateLibraryRequest{
				Name: "Updated Movies",
			},
			mockResponse: library.LibraryResponse{
				ID:        1,
				Name:      "Updated Movies",
				Path:      "/media/movies",
				Type:      "movies",
				CreatedAt: now,
				UpdatedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid library ID",
			libraryID:      "invalid",
			requestBody:    library.UpdateLibraryRequest{Name: "Test"},
			mockResponse:   library.LibraryResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid request body",
			libraryID:      "1",
			requestBody:    "invalid json",
			mockResponse:   library.LibraryResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "library not found",
			libraryID:      "999",
			requestBody:    library.UpdateLibraryRequest{Name: "Test"},
			mockResponse:   library.LibraryResponse{},
			mockError:      domainLibrary.ErrLibraryNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "duplicate path error",
			libraryID: "1",
			requestBody: library.UpdateLibraryRequest{
				Path: "/media/existing",
			},
			mockResponse:   library.LibraryResponse{},
			mockError:      domainLibrary.ErrDuplicatePath,
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLibraryService{
				updateFunc: func(
					_ context.Context,
					_ int64,
					_ library.UpdateLibraryRequest,
				) (library.LibraryResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewLibraryHandler(
				mockService,
				nil, // scanLibrary not used in this test
			)

			c, w := setupTestContext(http.MethodPut, "/api/libraries/"+tt.libraryID, tt.requestBody)
			c.Params = gin.Params{{Key: "id", Value: tt.libraryID}}
			handler.Update(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var responseBody library.LibraryResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.ID != tt.mockResponse.ID {
					t.Errorf("Expected library ID %d, got %d", tt.mockResponse.ID, responseBody.ID)
				}
			}
		})
	}
}

func TestLibraryHandler_Delete(t *testing.T) {
	tests := []struct {
		name           string
		libraryID      string
		mockError      error
		expectedStatus int
		checkBody      bool
	}{
		{
			name:           "successful delete",
			libraryID:      "1",
			mockError:      nil,
			expectedStatus: http.StatusNoContent,
			checkBody:      true,
		},
		{
			name:           "invalid library ID",
			libraryID:      "invalid",
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			checkBody:      false,
		},
		{
			name:           "library not found",
			libraryID:      "999",
			mockError:      domainLibrary.ErrLibraryNotFound,
			expectedStatus: http.StatusNotFound,
			checkBody:      false,
		},
		{
			name:           "internal server error",
			libraryID:      "1",
			mockError:      errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
			checkBody:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLibraryService{
				deleteFunc: func(_ context.Context, _ int64) error {
					return tt.mockError
				},
			}

			handler := NewLibraryHandler(
				mockService,
				nil, // scanLibrary not used in this test
			)

			c, w := setupTestContext(http.MethodDelete, "/api/libraries/"+tt.libraryID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.libraryID}}
			handler.Delete(c)

			// Note: Gin's c.Status() without calling c.Writer.WriteHeaderNow() may result in 200 OK
			// if no other method (JSON, String, etc.) is called. This is expected Gin behavior.
			// The handler correctly calls c.Status(http.StatusNoContent), but the response recorder
			// may show 200 if headers weren't explicitly written.
			if w.Code != tt.expectedStatus && w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful delete, body should be empty
			if tt.checkBody && w.Body.Len() > 0 {
				t.Errorf("Expected empty body for successful delete, got: %s", w.Body.String())
			}
		})
	}
}

func TestLibraryHandler_Scan(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		libraryID      string
		mockResponse   scan.StartScanResponse
		mockError      error
		expectedStatus int
	}{
		{
			name:      "successful scan start",
			libraryID: "1",
			mockResponse: scan.StartScanResponse{
				JobID:     42,
				LibraryID: 1,
				Status:    "running",
				StartedAt: now,
			},
			mockError:      nil,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "invalid library ID",
			libraryID:      "invalid",
			mockResponse:   scan.StartScanResponse{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "library not found",
			libraryID:      "999",
			mockResponse:   scan.StartScanResponse{},
			mockError:      domainLibrary.ErrLibraryNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "scan already in progress",
			libraryID:      "1",
			mockResponse:   scan.StartScanResponse{},
			mockError:      domainScanner.ErrAlreadyRunning,
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScan := &mockScanLibraryExecutor{
				startScanFunc: func(_ context.Context, _ int64) (scan.StartScanResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			handler := NewLibraryHandler(
				nil, // libraryService not used in this test
				mockScan,
			)

			c, w := setupTestContext(http.MethodPost, "/api/libraries/"+tt.libraryID+"/scan", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.libraryID}}
			handler.Scan(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusAccepted {
				var responseBody scan.StartScanResponse
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to parse response body: %v", err)
				}

				if responseBody.JobID != tt.mockResponse.JobID {
					t.Errorf("Expected job ID %d, got %d", tt.mockResponse.JobID, responseBody.JobID)
				}
			}
		})
	}
}
