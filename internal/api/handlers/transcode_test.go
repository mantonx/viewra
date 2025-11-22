package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	transcodeDomain "github.com/mantonx/viewra/internal/domain/transcode"
)

// MockTranscodeRepository implements transcode.Repository for testing
type MockTranscodeRepository struct {
	jobs      map[string]*transcodeDomain.TranscodeJob
	nextID    int64
	createErr error
	updateErr error
}

func NewMockTranscodeRepository() *MockTranscodeRepository {
	return &MockTranscodeRepository{
		jobs:   make(map[string]*transcodeDomain.TranscodeJob),
		nextID: 1,
	}
}

func (m *MockTranscodeRepository) makeKey(mediaID int64, quality string) string {
	return string(rune(mediaID)) + ":" + quality
}

func (m *MockTranscodeRepository) Create(ctx context.Context, job *transcodeDomain.TranscodeJob) error {
	if m.createErr != nil {
		return m.createErr
	}
	job.ID = m.nextID
	m.nextID++
	key := m.makeKey(job.MediaID, job.Quality)
	m.jobs[key] = job
	return nil
}

func (m *MockTranscodeRepository) Update(ctx context.Context, job *transcodeDomain.TranscodeJob) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	key := m.makeKey(job.MediaID, job.Quality)
	m.jobs[key] = job
	return nil
}

func (m *MockTranscodeRepository) GetByID(ctx context.Context, id int64) (*transcodeDomain.TranscodeJob, error) {
	for _, job := range m.jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, transcodeDomain.ErrJobNotFound
}

func (m *MockTranscodeRepository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcodeDomain.TranscodeJob, error) {
	key := m.makeKey(mediaID, quality)
	job, exists := m.jobs[key]
	if !exists {
		return nil, transcodeDomain.ErrJobNotFound
	}
	return job, nil
}

func (m *MockTranscodeRepository) Delete(ctx context.Context, id int64) error {
	for key, job := range m.jobs {
		if job.ID == id {
			delete(m.jobs, key)
			return nil
		}
	}
	return transcodeDomain.ErrJobNotFound
}

func (m *MockTranscodeRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	for key, job := range m.jobs {
		if job.MediaID == mediaID {
			delete(m.jobs, key)
		}
	}
	return nil
}

func (m *MockTranscodeRepository) ListByStatus(ctx context.Context, status string) ([]*transcodeDomain.TranscodeJob, error) {
	var result []*transcodeDomain.TranscodeJob
	for _, job := range m.jobs {
		if job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

func (m *MockTranscodeRepository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcodeDomain.TranscodeJob, error) {
	var result []*transcodeDomain.TranscodeJob
	for _, job := range m.jobs {
		if job.Status == transcodeDomain.StatusQueued {
			result = append(result, job)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockTranscodeRepository) ListProcessingJobs(ctx context.Context) ([]*transcodeDomain.TranscodeJob, error) {
	return m.ListByStatus(ctx, transcodeDomain.StatusProcessing)
}

func (m *MockTranscodeRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	count := int64(0)
	for _, job := range m.jobs {
		if job.Status == status {
			count++
		}
	}
	return count, nil
}

func (m *MockTranscodeRepository) GetTotalSize(ctx context.Context) (int64, error) {
	var totalSize int64
	// TODO: OutputSize field doesn't exist on TranscodeJob
	// for _, job := range m.jobs {
	// 	totalSize += job.OutputSize
	// }
	return totalSize, nil
}

func (m *MockTranscodeRepository) ListAll(ctx context.Context) ([]*transcodeDomain.TranscodeJob, error) {
	var result []*transcodeDomain.TranscodeJob
	for _, job := range m.jobs {
		result = append(result, job)
	}
	return result, nil
}

func (m *MockTranscodeRepository) ListByLRU(ctx context.Context, limit int) ([]*transcodeDomain.TranscodeJob, error) {
	// For testing, just return jobs in arbitrary order
	var result []*transcodeDomain.TranscodeJob
	for _, job := range m.jobs {
		result = append(result, job)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockTranscodeRepository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	// For testing, we don't need to track access times
	return nil
}

// MockMediaRepository for testing
type MockMediaRepository struct {
	mediaItems map[int64]*media.Media
}

func NewMockMediaRepository() *MockMediaRepository {
	return &MockMediaRepository{
		mediaItems: make(map[int64]*media.Media),
	}
}

func (m *MockMediaRepository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	item, exists := m.mediaItems[id]
	if !exists {
		return nil, media.ErrMediaNotFound
	}
	return item, nil
}

func (m *MockMediaRepository) Create(ctx context.Context, media *media.Media) error {
	return nil
}

func (m *MockMediaRepository) Update(ctx context.Context, media *media.Media) error {
	return nil
}

func (m *MockMediaRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *MockMediaRepository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	return nil, media.ErrMediaNotFound
}

func (m *MockMediaRepository) ListAll(ctx context.Context) ([]*media.Media, error) {
	return nil, nil
}

func (m *MockMediaRepository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	return nil, nil
}

func (m *MockMediaRepository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	return nil, nil
}

func (m *MockMediaRepository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	return false, nil
}

func (m *MockMediaRepository) Count(ctx context.Context, libraryID int64) (int64, error) {
	return 0, nil
}

func (m *MockMediaRepository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	return 0, nil
}

// MockLibraryRepository for testing
type MockLibraryRepository struct{}

func (m *MockLibraryRepository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	return nil, library.ErrLibraryNotFound
}

func (m *MockLibraryRepository) Create(ctx context.Context, library *library.Library) error {
	return nil
}

func (m *MockLibraryRepository) Update(ctx context.Context, library *library.Library) error {
	return nil
}

func (m *MockLibraryRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *MockLibraryRepository) List(ctx context.Context) ([]*library.Library, error) {
	return nil, nil
}

func (m *MockLibraryRepository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	return nil, library.ErrLibraryNotFound
}

func (m *MockLibraryRepository) Exists(ctx context.Context, path string) (bool, error) {
	return false, nil
}

// setupTestHandler creates a test handler with mocked dependencies
func setupTestHandler(t *testing.T) (*TranscodeHandler, *MockTranscodeRepository, *transcode.Queue) {
	repo := NewMockTranscodeRepository()
	mediaRepo := NewMockMediaRepository()
	libraryRepo := &MockLibraryRepository{}

	// Create a minimal queue for testing
	queueConfig := &transcode.QueueConfig{
		WorkerCount:   1,
		OutputBaseDir: t.TempDir(),
	}
	queue := transcode.NewQueue(queueConfig, repo, nil, nil)

	// Create use cases
	createJobUC := transcode.NewCreateJobUseCase(repo, queue)
	getStatusUC := transcode.NewGetJobStatusUseCase(repo)
	serveManifestUC := transcode.NewServeManifestUseCase(repo, mediaRepo, libraryRepo, createJobUC)

	handler := NewTranscodeHandler(
		createJobUC,
		getStatusUC,
		serveManifestUC,
		queue,
		nil, // CleanupService not needed for these tests
		t.TempDir(),
	)

	return handler, repo, queue
}

func TestCreateTranscodeJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		requestBody    map[string]interface{}
		setupRepo      func(*MockTranscodeRepository)
		expectedStatus int
		expectJobID    bool
	}{
		{
			name:    "Successfully create job",
			mediaID: "123",
			requestBody: map[string]interface{}{
				"quality": "720p",
			},
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusCreated,
			expectJobID:    true,
		},
		{
			name:    "Invalid media ID",
			mediaID: "invalid",
			requestBody: map[string]interface{}{
				"quality": "720p",
			},
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectJobID:    false,
		},
		{
			name:    "Invalid quality",
			mediaID: "123",
			requestBody: map[string]interface{}{
				"quality": "invalid",
			},
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectJobID:    false,
		},
		{
			name:           "Missing quality in request",
			mediaID:        "123",
			requestBody:    map[string]interface{}{},
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectJobID:    false,
		},
		{
			name:    "Job already exists",
			mediaID: "123",
			requestBody: map[string]interface{}{
				"quality": "720p",
			},
			setupRepo: func(r *MockTranscodeRepository) {
				job, _ := transcodeDomain.NewTranscodeJob(123, "720p", transcodeDomain.TypeTranscode)
				r.Create(context.Background(), job)
			},
			expectedStatus: http.StatusConflict,
			expectJobID:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, _ := setupTestHandler(t)
			tt.setupRepo(repo)

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/media/"+tt.mediaID+"/transcode", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: tt.mediaID}}

			// Call handler
			handler.CreateTranscodeJob(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Check response body for successful requests
			if tt.expectJobID {
				var response TranscodeJobResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if response.ID == 0 {
					t.Error("Expected job ID in response")
				}
				if response.MediaID != 123 {
					t.Errorf("Expected media_id 123, got %d", response.MediaID)
				}
				if response.Quality != "720p" {
					t.Errorf("Expected quality 720p, got %s", response.Quality)
				}
			}
		})
	}
}

func TestGetTranscodeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		quality        string
		setupRepo      func(*MockTranscodeRepository)
		expectedStatus int
		expectJob      bool
	}{
		{
			name:    "Get existing job",
			mediaID: "123",
			quality: "720p",
			setupRepo: func(r *MockTranscodeRepository) {
				job, _ := transcodeDomain.NewTranscodeJob(123, "720p", transcodeDomain.TypeTranscode)
				r.Create(context.Background(), job)
			},
			expectedStatus: http.StatusOK,
			expectJob:      true,
		},
		{
			name:           "Job not found",
			mediaID:        "123",
			quality:        "720p",
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusNotFound,
			expectJob:      false,
		},
		{
			name:           "Invalid media ID",
			mediaID:        "invalid",
			quality:        "720p",
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectJob:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, _ := setupTestHandler(t)
			tt.setupRepo(repo)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/media/"+tt.mediaID+"/transcode/"+tt.quality, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.mediaID},
				{Key: "quality", Value: tt.quality},
			}

			// Call handler
			handler.GetTranscodeStatus(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Check response for successful requests
			if tt.expectJob {
				var response TranscodeJobResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if response.ID == 0 {
					t.Error("Expected job ID in response")
				}
				if response.Status != transcodeDomain.StatusQueued {
					t.Errorf("Expected status %s, got %s", transcodeDomain.StatusQueued, response.Status)
				}
			}
		})
	}
}

func TestGetQueueStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupRepo      func(*MockTranscodeRepository)
		expectedStatus int
		expectStats    bool
	}{
		{
			name:           "Get stats with no jobs",
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusOK,
			expectStats:    true,
		},
		{
			name: "Get stats with multiple jobs",
			setupRepo: func(r *MockTranscodeRepository) {
				job1, _ := transcodeDomain.NewTranscodeJob(123, "720p", transcodeDomain.TypeTranscode)
				r.Create(context.Background(), job1)

				job2, _ := transcodeDomain.NewTranscodeJob(124, "1080p", transcodeDomain.TypeTranscode)
				job2.MarkAsProcessing()
				r.Create(context.Background(), job2)

				job3, _ := transcodeDomain.NewTranscodeJob(125, "720p", transcodeDomain.TypeTranscode)
				job3.MarkAsCompleted()
				r.Create(context.Background(), job3)
			},
			expectedStatus: http.StatusOK,
			expectStats:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, _ := setupTestHandler(t)
			tt.setupRepo(repo)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/transcode/stats", nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.GetQueueStats(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Check response
			if tt.expectStats {
				var stats transcode.QueueStats
				if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if stats.WorkerCount != 1 {
					t.Errorf("Expected worker count 1, got %d", stats.WorkerCount)
				}
			}
		})
	}
}

func TestServeManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		quality        string
		setupEnv       func(*testing.T, string) // Setup files and media repo
		expectedStatus int
		expectFile     bool
		expectJSON     bool
		skip           bool // Skip this test case
	}{
		{
			name:    "Serve existing manifest",
			mediaID: "123",
			quality: "720p",
			setupEnv: func(t *testing.T, outputDir string) {
				// Create manifest file
				manifestDir := filepath.Join(outputDir, "dash", "123", "720p")
				os.MkdirAll(manifestDir, 0755)
				manifestPath := filepath.Join(manifestDir, "manifest.mpd")
				os.WriteFile(manifestPath, []byte("<MPD>test manifest</MPD>"), 0644)
				// Create segments
				os.WriteFile(filepath.Join(manifestDir, "segment_0.m4s"), []byte("segment"), 0644)
				os.WriteFile(filepath.Join(manifestDir, "segment_1.m4s"), []byte("segment"), 0644)
			},
			expectedStatus: http.StatusOK,
			expectFile:     true,
			expectJSON:     false,
			skip:           true, // TODO: Need to setup media repository properly
		},
		{
			name:           "Invalid media ID",
			mediaID:        "invalid",
			quality:        "720p",
			setupEnv:       func(t *testing.T, outputDir string) {},
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
			expectJSON:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test - needs proper media repository setup")
			}

			handler, _, _ := setupTestHandler(t)

			// Setup environment
			if tt.setupEnv != nil {
				tt.setupEnv(t, handler.outputDir)
			}

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/media/"+tt.mediaID+"/dash/"+tt.quality+"/manifest.mpd", nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.mediaID},
				{Key: "quality", Value: tt.quality},
			}

			// Call handler
			handler.ServePlaylist(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Check content type for file responses
			if tt.expectFile {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/dash+xml" {
					t.Errorf("Expected Content-Type application/dash+xml, got %s", contentType)
				}
			}
		})
	}
}

func TestCancelTranscodeJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		quality        string
		setupRepo      func(*MockTranscodeRepository)
		expectedStatus int
	}{
		{
			name:           "Invalid media ID",
			mediaID:        "invalid",
			quality:        "720p",
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Job not found",
			mediaID:        "123",
			quality:        "720p",
			setupRepo:      func(r *MockTranscodeRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "Job not processing",
			mediaID: "123",
			quality: "720p",
			setupRepo: func(r *MockTranscodeRepository) {
				job, _ := transcodeDomain.NewTranscodeJob(123, "720p", transcodeDomain.TypeTranscode)
				r.Create(context.Background(), job)
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, _ := setupTestHandler(t)
			tt.setupRepo(repo)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/api/media/"+tt.mediaID+"/transcode/"+tt.quality+"/cancel", nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.mediaID},
				{Key: "quality", Value: tt.quality},
			}

			// Call handler
			handler.CancelTranscodeJob(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestServeDASHSegment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		mediaID        string
		quality        string
		filename       string
		setupEnv       func(*testing.T, string)
		expectedStatus int
		skip           bool
	}{
		{
			name:     "Serve existing segment",
			mediaID:  "123",
			quality:  "720p",
			filename: "segment_0.m4s",
			setupEnv: func(t *testing.T, outputDir string) {
				segmentDir := filepath.Join(outputDir, "dash", "123", "720p")
				os.MkdirAll(segmentDir, 0755)
				segmentPath := filepath.Join(segmentDir, "segment_0.m4s")
				os.WriteFile(segmentPath, []byte("segment data"), 0644)
			},
			expectedStatus: http.StatusOK,
			skip:           true, // TODO: Needs proper setup
		},
		{
			name:           "Segment not found",
			mediaID:        "123",
			quality:        "720p",
			filename:       "nonexistent.m4s",
			setupEnv:       func(t *testing.T, outputDir string) {},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test - needs proper setup")
			}

			handler, _, _ := setupTestHandler(t)

			// Setup environment
			if tt.setupEnv != nil {
				tt.setupEnv(t, handler.outputDir)
			}

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/media/"+tt.mediaID+"/dash/"+tt.quality+"/"+tt.filename, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.mediaID},
				{Key: "quality", Value: tt.quality},
				{Key: "filename", Value: tt.filename},
			}

			// Call handler
			handler.ServeHLSSegment(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check content type for successful requests
			if tt.expectedStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/octet-stream" {
					t.Errorf("Expected Content-Type application/octet-stream, got %s", contentType)
				}
			}
		})
	}
}
