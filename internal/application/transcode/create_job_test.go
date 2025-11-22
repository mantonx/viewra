package transcode

import (
	"context"
	"errors"
	"testing"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// MockRepository is a mock implementation of transcode.Repository for testing
type MockRepository struct {
	jobs      map[string]*transcode.TranscodeJob // key: "mediaID:quality"
	nextID    int64
	createErr error
	getErr    error
	deleteErr error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		jobs:   make(map[string]*transcode.TranscodeJob),
		nextID: 1,
	}
}

func (m *MockRepository) makeKey(mediaID int64, quality string) string {
	return string(rune(mediaID)) + ":" + quality
}

func (m *MockRepository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	if m.createErr != nil {
		return m.createErr
	}
	job.ID = m.nextID
	m.nextID++
	key := m.makeKey(job.MediaID, job.Quality)
	m.jobs[key] = job
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, id int64) (*transcode.TranscodeJob, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, job := range m.jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, transcode.ErrJobNotFound
}

func (m *MockRepository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	key := m.makeKey(mediaID, quality)
	job, exists := m.jobs[key]
	if !exists {
		return nil, transcode.ErrJobNotFound
	}
	return job, nil
}

func (m *MockRepository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	key := m.makeKey(job.MediaID, job.Quality)
	m.jobs[key] = job
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for key, job := range m.jobs {
		if job.ID == id {
			delete(m.jobs, key)
			return nil
		}
	}
	return transcode.ErrJobNotFound
}

func (m *MockRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for key, job := range m.jobs {
		if job.MediaID == mediaID {
			delete(m.jobs, key)
		}
	}
	return nil
}

func (m *MockRepository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	var result []*transcode.TranscodeJob
	for _, job := range m.jobs {
		if job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

func (m *MockRepository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	var result []*transcode.TranscodeJob
	for _, job := range m.jobs {
		if job.Status == transcode.StatusQueued {
			result = append(result, job)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockRepository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	return m.ListByStatus(ctx, transcode.StatusProcessing)
}

func (m *MockRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	count := int64(0)
	for _, job := range m.jobs {
		if job.Status == status {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) GetTotalSize(ctx context.Context) (int64, error) {
	return int64(len(m.jobs)), nil
}

func (m *MockRepository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	var result []*transcode.TranscodeJob
	for _, job := range m.jobs {
		result = append(result, job)
	}
	return result, nil
}

func (m *MockRepository) ListByLRU(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	var result []*transcode.TranscodeJob
	for _, job := range m.jobs {
		result = append(result, job)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockRepository) UpdateAccess(ctx context.Context, id int64, quality string) error {
	// Mock implementation - just return nil
	return nil
}

func TestCreateJob(t *testing.T) {
	tests := []struct {
		name        string
		req         CreateJobRequest
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Successfully create job",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeTranscode,
			},
			setupMock:   func(m *MockRepository) {},
			expectError: false,
		},
		{
			name: "Invalid quality",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: "invalid_quality",
				Type:    transcode.TypeTranscode,
			},
			setupMock:   func(m *MockRepository) {},
			expectError: true,
			errorMsg:    "invalid quality",
		},
		{
			name: "Job already queued",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeTranscode,
			},
			setupMock: func(m *MockRepository) {
				job, _ := transcode.NewTranscodeJob(123, transcode.Quality720p, transcode.TypeTranscode)
				job.Status = transcode.StatusQueued
				m.Create(context.Background(), job)
			},
			expectError: true,
			errorMsg:    "already exists",
		},
		{
			name: "Job already processing",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeTranscode,
			},
			setupMock: func(m *MockRepository) {
				job, _ := transcode.NewTranscodeJob(123, transcode.Quality720p, transcode.TypeTranscode)
				job.MarkAsProcessing()
				m.Create(context.Background(), job)
			},
			expectError: true,
			errorMsg:    "already exists",
		},
		{
			name: "Job already completed",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeTranscode,
			},
			setupMock: func(m *MockRepository) {
				job, _ := transcode.NewTranscodeJob(123, transcode.Quality720p, transcode.TypeTranscode)
				job.MarkAsCompleted()
				m.Create(context.Background(), job)
			},
			expectError: true,
			errorMsg:    "already exists",
		},
		{
			name: "Recreate failed job",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeTranscode,
			},
			setupMock: func(m *MockRepository) {
				job, _ := transcode.NewTranscodeJob(123, transcode.Quality720p, transcode.TypeTranscode)
				job.MarkAsFailed("previous error")
				m.Create(context.Background(), job)
			},
			expectError: false, // Should succeed by deleting old job
		},
		{
			name: "Default to transcode type when not specified",
			req: CreateJobRequest{
				MediaID: 123,
				Quality: transcode.Quality1080p,
				Type:    "", // Empty type
			},
			setupMock:   func(m *MockRepository) {},
			expectError: false,
		},
		{
			name: "Create remux job",
			req: CreateJobRequest{
				MediaID: 456,
				Quality: transcode.Quality720p,
				Type:    transcode.TypeRemux,
			},
			setupMock:   func(m *MockRepository) {},
			expectError: false,
		},
		{
			name: "Create remux_audio job",
			req: CreateJobRequest{
				MediaID: 789,
				Quality: transcode.Quality4K,
				Type:    transcode.TypeRemuxAudio,
			},
			setupMock:   func(m *MockRepository) {},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			tt.setupMock(repo)

			job, err := CreateJob(context.Background(), repo, tt.req)

			if tt.expectError {
				if err == nil {
					t.Errorf("CreateJob() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !containsStr(err.Error(), tt.errorMsg) {
					t.Errorf("CreateJob() error = %v, want to contain %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("CreateJob() unexpected error = %v", err)
					return
				}
				if job == nil {
					t.Errorf("CreateJob() returned nil job")
					return
				}
				if job.MediaID != tt.req.MediaID {
					t.Errorf("CreateJob() job.MediaID = %v, want %v", job.MediaID, tt.req.MediaID)
				}
				if job.Quality != tt.req.Quality {
					t.Errorf("CreateJob() job.Quality = %v, want %v", job.Quality, tt.req.Quality)
				}
				expectedType := tt.req.Type
				if expectedType == "" {
					expectedType = transcode.TypeTranscode
				}
				if job.Type != expectedType {
					t.Errorf("CreateJob() job.Type = %v, want %v", job.Type, expectedType)
				}
				if job.Status != transcode.StatusQueued {
					t.Errorf("CreateJob() job.Status = %v, want %v", job.Status, transcode.StatusQueued)
				}
			}
		})
	}
}

func TestIsValidQuality(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		want    bool
	}{
		{"360p is valid", transcode.Quality360p, true},
		{"720p is valid", transcode.Quality720p, true},
		{"1080p is valid", transcode.Quality1080p, true},
		{"4k is valid", transcode.Quality4K, true},
		{"invalid quality", "invalid", false},
		{"empty quality", "", false},
		{"uppercase 720P", "720P", false},
		{"480p not supported", "480p", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidQuality(tt.quality)
			if got != tt.want {
				t.Errorf("isValidQuality(%v) = %v, want %v", tt.quality, got, tt.want)
			}
		})
	}
}

func TestCreateJobRepositoryErrors(t *testing.T) {
	t.Run("Create returns error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createErr = errors.New("database error")

		_, err := CreateJob(context.Background(), repo, CreateJobRequest{
			MediaID: 123,
			Quality: transcode.Quality720p,
			Type:    transcode.TypeTranscode,
		})

		if err == nil {
			t.Error("CreateJob() expected error but got none")
		}
	})

	t.Run("Delete failed job returns error", func(t *testing.T) {
		repo := NewMockRepository()

		// Create a failed job
		job, _ := transcode.NewTranscodeJob(123, transcode.Quality720p, transcode.TypeTranscode)
		job.MarkAsFailed("previous error")
		repo.Create(context.Background(), job)

		// Make delete fail
		repo.deleteErr = errors.New("delete error")

		_, err := CreateJob(context.Background(), repo, CreateJobRequest{
			MediaID: 123,
			Quality: transcode.Quality720p,
			Type:    transcode.TypeTranscode,
		})

		if err == nil {
			t.Error("CreateJob() expected error but got none")
		}
		if !containsStr(err.Error(), "delete") {
			t.Errorf("CreateJob() error = %v, want to contain 'delete'", err.Error())
		}
	})
}

// Helper function to check if a string contains a substring
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || containsStr(s[1:], substr)))
}
