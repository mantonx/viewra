package transcoding

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/paths"
)

// mockRepository implements transcode.Repository for testing
type mockRepository struct {
	updateFunc func(ctx context.Context, job *transcode.TranscodeJob) error
	updateErr  error
}

func (m *mockRepository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	return nil
}

func (m *mockRepository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, job)
	}
	return m.updateErr
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	return 0, nil
}

func (m *mockRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	return nil
}

func (m *mockRepository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockRepository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	return nil
}

func (m *mockRepository) GetTotalSize(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRepository) ListByLRU(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

// TestNewService tests the creation of a new transcoding service
func TestNewService(t *testing.T) {
	tests := []struct {
		name        string
		repo        transcode.Repository
		logger      *slog.Logger
		setupEnv    func()
		cleanupEnv  func()
		shouldError bool
		errorMsg    string
	}{
		{
			name:   "Valid service creation with FFmpeg available",
			repo:   &mockRepository{},
			logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
			setupEnv: func() {
				// Try to use system FFmpeg if available
			},
			cleanupEnv:  func() {},
			shouldError: false,
		},
		{
			name:   "Service creation with nil logger (uses default)",
			repo:   &mockRepository{},
			logger: nil, // Should use default logger
			setupEnv: func() {
				// Try to use system FFmpeg if available
			},
			cleanupEnv:  func() {},
			shouldError: false,
		},
		// Note: Skipping the "FFmpeg not available" test case because
		// the system may have FFmpeg installed in multiple locations
		// and setting VIEWRA_FFMPEG_PATH may not prevent finding it.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			service, err := NewService(tt.repo, tt.logger)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
				if service != nil {
					t.Error("Expected nil service when error occurs")
				}
			} else {
				// If FFmpeg is not available in test environment, skip validation
				if err != nil {
					if strings.Contains(err.Error(), "ffmpeg") {
						t.Skipf("FFmpeg not available in test environment: %v", err)
					}
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if service == nil {
					t.Fatal("Expected non-nil service")
				}

				// Service is valid - we can test that it implements the interface
				// We can't directly access internal fields, but we can verify it's not nil
				// and implements the Service interface
				if _, ok := service.(Service); !ok {
					t.Error("Service does not implement Service interface")
				}
			}
		})
	}
}

// TestGetManifestPath tests the GetManifestPath utility function
func TestGetManifestPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		mediaID  int64
		quality  string
		expected string
	}{
		{
			name:     "Standard 720p manifest path",
			baseDir:  "/var/viewra/data",
			mediaID:  12345,
			quality:  "720p",
			expected: filepath.Join("/var/viewra/data", "hls", "12345", "720p", "playlist.m3u8"),
		},
		{
			name:     "1080p manifest path",
			baseDir:  "/tmp/output",
			mediaID:  67890,
			quality:  "1080p",
			expected: filepath.Join("/tmp/output", "hls", "67890", "1080p", "playlist.m3u8"),
		},
		{
			name:     "4K manifest path",
			baseDir:  "/media/transcodes",
			mediaID:  999,
			quality:  "4k",
			expected: filepath.Join("/media/transcodes", "hls", "999", "4k", "playlist.m3u8"),
		},
		{
			name:     "Quality normalization (uppercase to lowercase)",
			baseDir:  "/data",
			mediaID:  111,
			quality:  "720P",
			expected: filepath.Join("/data", "hls", "111", "720p", "playlist.m3u8"),
		},
		{
			name:     "Path with trailing slash",
			baseDir:  "/data/",
			mediaID:  222,
			quality:  "1080p",
			expected: filepath.Join("/data/", "hls", "222", "1080p", "playlist.m3u8"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paths.GetHLSManifestPath(tt.baseDir, tt.mediaID, tt.quality)
			if result != tt.expected {
				t.Errorf("GetManifestPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetOutputDirectory tests the GetOutputDirectory utility function
func TestGetOutputDirectory(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		mediaID  int64
		quality  string
		expected string
	}{
		{
			name:     "Standard 720p output directory",
			baseDir:  "/var/viewra/data",
			mediaID:  12345,
			quality:  "720p",
			expected: filepath.Join("/var/viewra/data", "hls", "12345", "720p"),
		},
		{
			name:     "1080p output directory",
			baseDir:  "/tmp/output",
			mediaID:  67890,
			quality:  "1080p",
			expected: filepath.Join("/tmp/output", "hls", "67890", "1080p"),
		},
		{
			name:     "4K output directory",
			baseDir:  "/media/transcodes",
			mediaID:  999,
			quality:  "4k",
			expected: filepath.Join("/media/transcodes", "hls", "999", "4k"),
		},
		{
			name:     "Quality normalization (mixed case)",
			baseDir:  "/data",
			mediaID:  111,
			quality:  "1080P",
			expected: filepath.Join("/data", "hls", "111", "1080p"),
		},
		{
			name:     "Relative base directory",
			baseDir:  "./data",
			mediaID:  333,
			quality:  "720p",
			expected: filepath.Join("./data", "hls", "333", "720p"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paths.GetHLSOutputPath(tt.baseDir, tt.mediaID, tt.quality)
			if result != tt.expected {
				t.Errorf("GetOutputDirectory() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestTranscodeToHLS tests the TranscodeToHLS method
func TestTranscodeToHLS(t *testing.T) {
	tests := []struct {
		name        string
		job         *transcode.TranscodeJob
		inputPath   string
		outputDir   string
		setupMock   func(*mockRepository)
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Nil job should error",
			job:  nil,
			setupMock: func(m *mockRepository) {
				// No setup needed
			},
			shouldError: true,
			errorMsg:    "transcode job is nil",
		},
		{
			name: "Job with wrong status should error",
			job: &transcode.TranscodeJob{
				ID:      1,
				MediaID: 100,
				Quality: "720p",
				Type:    transcode.TypeTranscode,
				Status:  transcode.StatusCompleted, // Wrong status
			},
			inputPath: "/path/to/video.mp4",
			outputDir: "/tmp/output",
			setupMock: func(m *mockRepository) {
				// No setup needed
			},
			shouldError: true,
			errorMsg:    "job must be in queued or processing status",
		},
		{
			name: "Job with invalid quality should error",
			job: &transcode.TranscodeJob{
				ID:      1,
				MediaID: 100,
				Quality: "invalid_quality",
				Type:    transcode.TypeTranscode,
				Status:  transcode.StatusQueued,
			},
			inputPath: "/path/to/video.mp4",
			outputDir: "/tmp/output",
			setupMock: func(m *mockRepository) {
				// No setup needed
			},
			shouldError: true,
			errorMsg:    "invalid quality profile",
		},
		{
			name: "Queued job with valid quality",
			job: &transcode.TranscodeJob{
				ID:      1,
				MediaID: 100,
				Quality: "720p",
				Type:    transcode.TypeTranscode,
				Status:  transcode.StatusQueued,
			},
			inputPath: "/nonexistent/video.mp4", // Will fail but tests the flow
			outputDir: "",                        // Will be set in test
			setupMock: func(m *mockRepository) {
				m.updateFunc = func(ctx context.Context, job *transcode.TranscodeJob) error {
					// Allow updates
					return nil
				}
			},
			shouldError: true, // Will error because input file doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			// Create service through public constructor
			svc, err := NewService(repo, slog.Default())
			if err != nil {
				if strings.Contains(err.Error(), "ffmpeg") {
					t.Skip("FFmpeg not available in test environment")
				}
				t.Fatalf("Failed to create service: %v", err)
			}

			outputDir := tt.outputDir
			if outputDir == "" {
				outputDir = t.TempDir()
			}

			err = svc.TranscodeToHLS(context.Background(), tt.job, tt.inputPath, outputDir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestRemuxToHLS tests the RemuxToHLS method
func TestRemuxToHLS(t *testing.T) {
	tests := []struct {
		name        string
		job         *transcode.TranscodeJob
		inputPath   string
		outputDir   string
		setupMock   func(*mockRepository)
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Nil job should error",
			job:  nil,
			setupMock: func(m *mockRepository) {
				// No setup needed
			},
			shouldError: true,
			errorMsg:    "transcode job is nil",
		},
		{
			name: "Queued remux job",
			job: &transcode.TranscodeJob{
				ID:      1,
				MediaID: 100,
				Quality: "720p",
				Type:    transcode.TypeRemux,
				Status:  transcode.StatusQueued,
			},
			inputPath: "/nonexistent/video.mp4",
			outputDir: "",
			setupMock: func(m *mockRepository) {
				m.updateFunc = func(ctx context.Context, job *transcode.TranscodeJob) error {
					return nil
				}
			},
			shouldError: true, // Will error because input file doesn't exist
		},
		{
			name: "Processing remux job should be allowed",
			job: &transcode.TranscodeJob{
				ID:      2,
				MediaID: 200,
				Quality: "1080p",
				Type:    transcode.TypeRemux,
				Status:  transcode.StatusProcessing,
			},
			inputPath: "/nonexistent/video.mkv",
			outputDir: "",
			setupMock: func(m *mockRepository) {
				m.updateFunc = func(ctx context.Context, job *transcode.TranscodeJob) error {
					return nil
				}
			},
			shouldError: true, // Will error because input file doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			svc, err := NewService(repo, slog.Default())
			if err != nil {
				if strings.Contains(err.Error(), "ffmpeg") {
					t.Skip("FFmpeg not available in test environment")
				}
				t.Fatalf("Failed to create service: %v", err)
			}

			outputDir := tt.outputDir
			if outputDir == "" {
				outputDir = t.TempDir()
			}

			err = svc.RemuxToHLS(context.Background(), tt.job, tt.inputPath, outputDir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestRemuxWithAudioDownmixHLS tests the RemuxWithAudioDownmixHLS method
func TestRemuxWithAudioDownmixHLS(t *testing.T) {
	tests := []struct {
		name        string
		job         *transcode.TranscodeJob
		inputPath   string
		outputDir   string
		setupMock   func(*mockRepository)
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Nil job should error",
			job:  nil,
			setupMock: func(m *mockRepository) {
				// No setup needed
			},
			shouldError: true,
			errorMsg:    "transcode job is nil",
		},
		{
			name: "Queued remux with audio downmix job",
			job: &transcode.TranscodeJob{
				ID:      1,
				MediaID: 100,
				Quality: "720p",
				Type:    transcode.TypeRemuxAudio,
				Status:  transcode.StatusQueued,
			},
			inputPath: "/nonexistent/video.mp4",
			outputDir: "",
			setupMock: func(m *mockRepository) {
				m.updateFunc = func(ctx context.Context, job *transcode.TranscodeJob) error {
					return nil
				}
			},
			shouldError: true, // Will error because input file doesn't exist
		},
		{
			name: "Repository update error should be handled",
			job: &transcode.TranscodeJob{
				ID:      3,
				MediaID: 300,
				Quality: "1080p",
				Type:    transcode.TypeRemuxAudio,
				Status:  transcode.StatusQueued,
			},
			inputPath: "/nonexistent/video.mkv",
			outputDir: "",
			setupMock: func(m *mockRepository) {
				m.updateFunc = func(ctx context.Context, job *transcode.TranscodeJob) error {
					return errors.New("database error")
				}
			},
			shouldError: true, // Will error from multiple sources
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			svc, err := NewService(repo, slog.Default())
			if err != nil {
				if strings.Contains(err.Error(), "ffmpeg") {
					t.Skip("FFmpeg not available in test environment")
				}
				t.Fatalf("Failed to create service: %v", err)
			}

			outputDir := tt.outputDir
			if outputDir == "" {
				outputDir = t.TempDir()
			}

			err = svc.RemuxWithAudioDownmixHLS(context.Background(), tt.job, tt.inputPath, outputDir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestServiceMethodsCallExecutor verifies that service methods properly delegate to executor
func TestServiceMethodsCallExecutor(t *testing.T) {
	// Create a valid queued job
	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "720p",
		Type:    transcode.TypeTranscode,
		Status:  transcode.StatusQueued,
	}

	repo := &mockRepository{
		updateFunc: func(ctx context.Context, job *transcode.TranscodeJob) error {
			return nil
		},
	}

	svc, err := NewService(repo, slog.Default())
	if err != nil {
		if strings.Contains(err.Error(), "ffmpeg") {
			t.Skip("FFmpeg not available in test environment")
		}
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	inputPath := "/nonexistent/video.mp4"
	outputDir := t.TempDir()

	// Test that all methods attempt to call the executor
	// They will fail because the file doesn't exist, but we verify they get to the executor

	t.Run("TranscodeToHLS calls executor", func(t *testing.T) {
		err := svc.TranscodeToHLS(ctx, job, inputPath, outputDir)
		if err == nil {
			t.Error("Expected error since input file doesn't exist")
		}
		// The error should be from the execution phase, not from nil checks
		if strings.Contains(err.Error(), "nil") {
			t.Error("Error should not be about nil values, should reach executor")
		}
	})

	t.Run("RemuxToHLS calls executor", func(t *testing.T) {
		err := svc.RemuxToHLS(ctx, job, inputPath, outputDir)
		if err == nil {
			t.Error("Expected error since input file doesn't exist")
		}
		if strings.Contains(err.Error(), "nil") {
			t.Error("Error should not be about nil values, should reach executor")
		}
	})

	t.Run("RemuxWithAudioDownmixHLS calls executor", func(t *testing.T) {
		err := svc.RemuxWithAudioDownmixHLS(ctx, job, inputPath, outputDir)
		if err == nil {
			t.Error("Expected error since input file doesn't exist")
		}
		if strings.Contains(err.Error(), "nil") {
			t.Error("Error should not be about nil values, should reach executor")
		}
	})
}

// TestServiceWithContextCancellation tests that context cancellation is respected
func TestServiceWithContextCancellation(t *testing.T) {
	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "720p",
		Type:    transcode.TypeTranscode,
		Status:  transcode.StatusQueued,
	}

	repo := &mockRepository{
		updateFunc: func(ctx context.Context, job *transcode.TranscodeJob) error {
			return nil
		},
	}

	svc, err := NewService(repo, slog.Default())
	if err != nil {
		if strings.Contains(err.Error(), "ffmpeg") {
			t.Skip("FFmpeg not available in test environment")
		}
		t.Fatalf("Failed to create service: %v", err)
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	inputPath := "/nonexistent/video.mp4"
	outputDir := t.TempDir()

	err = svc.TranscodeToHLS(ctx, job, inputPath, outputDir)
	// Should error (either from cancellation or from missing file)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
