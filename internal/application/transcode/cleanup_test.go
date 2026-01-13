package transcode

import (
	"testing"
	"time"
)

func TestNewFailedJobsFilter(t *testing.T) {
	olderThan := 24 * time.Hour
	filter := NewFailedJobsFilter(olderThan, true)

	if !filter.IncludeFailed {
		t.Error("NewFailedJobsFilter should set IncludeFailed = true")
	}
	if filter.IncludeCompleted {
		t.Error("NewFailedJobsFilter should not set IncludeCompleted")
	}
	if filter.IncludeQueued {
		t.Error("NewFailedJobsFilter should not set IncludeQueued")
	}
	if filter.OlderThan == nil || *filter.OlderThan != olderThan {
		t.Errorf("NewFailedJobsFilter OlderThan = %v, want %v", filter.OlderThan, olderThan)
	}
	if !filter.DryRun {
		t.Error("NewFailedJobsFilter should set DryRun = true")
	}
}

func TestNewSingleJobFilter(t *testing.T) {
	mediaID := int64(123)
	quality := "720p"
	filter := NewSingleJobFilter(mediaID, quality, false)

	if filter.MediaID == nil || *filter.MediaID != mediaID {
		t.Errorf("NewSingleJobFilter MediaID = %v, want %d", filter.MediaID, mediaID)
	}
	if filter.Quality == nil || *filter.Quality != quality {
		t.Errorf("NewSingleJobFilter Quality = %v, want %s", filter.Quality, quality)
	}
	if filter.DryRun {
		t.Error("NewSingleJobFilter should set DryRun = false")
	}
}

func TestNewOldJobsFilter(t *testing.T) {
	olderThan := 7 * 24 * time.Hour
	filter := NewOldJobsFilter(olderThan, true)

	if !filter.IncludeCompleted {
		t.Error("NewOldJobsFilter should set IncludeCompleted = true")
	}
	if !filter.IncludeFailed {
		t.Error("NewOldJobsFilter should set IncludeFailed = true")
	}
	if !filter.IncludeQueued {
		t.Error("NewOldJobsFilter should set IncludeQueued = true")
	}
	if filter.OlderThan == nil || *filter.OlderThan != olderThan {
		t.Errorf("NewOldJobsFilter OlderThan = %v, want %v", filter.OlderThan, olderThan)
	}
	if !filter.DryRun {
		t.Error("NewOldJobsFilter should set DryRun = true")
	}
}

func TestNewCleanupService(t *testing.T) {
	repo := NewMockRepository()
	outputDir := "/tmp/transcodes"

	service := NewCleanupService(repo, outputDir)

	if service == nil {
		t.Fatal("NewCleanupService returned nil")
	}
	if service.repo != repo {
		t.Error("NewCleanupService did not set repo correctly")
	}
	if service.outputDir != outputDir {
		t.Errorf("NewCleanupService outputDir = %q, want %q", service.outputDir, outputDir)
	}
}

func TestCleanupResultFields(t *testing.T) {
	result := CleanupResult{
		DeletedCount:     5,
		DeletedSizeBytes: 1024 * 1024 * 100, // 100 MB
		FailedCount:      1,
		Errors:           []error{},
		DeletedItems:     make([]CleanupItem, 0),
	}

	if result.DeletedCount != 5 {
		t.Errorf("DeletedCount = %d, want 5", result.DeletedCount)
	}
	if result.DeletedSizeBytes != 104857600 {
		t.Errorf("DeletedSizeBytes = %d, want 104857600", result.DeletedSizeBytes)
	}
	if result.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", result.FailedCount)
	}
}

func TestCleanupItemFields(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-time.Hour)

	item := CleanupItem{
		JobID:       1,
		MediaID:     100,
		Quality:     "1080p",
		Status:      "completed",
		SizeBytes:   1024 * 1024 * 500,
		Path:        "/tmp/transcodes/hls/100/1080p",
		CreatedAt:   now.Add(-24 * time.Hour),
		CompletedAt: &completedAt,
	}

	if item.JobID != 1 {
		t.Errorf("JobID = %d, want 1", item.JobID)
	}
	if item.Quality != "1080p" {
		t.Errorf("Quality = %q, want %q", item.Quality, "1080p")
	}
	if item.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestDiskUsageFields(t *testing.T) {
	usage := DiskUsage{
		OutputDir:       "/data/transcodes",
		TotalSizeBytes:  1024 * 1024 * 1024 * 10, // 10 GB
		FileCount:       500,
		TotalJobs:       50,
		CompletedCount:  40,
		FailedCount:     5,
		QueuedCount:     3,
		ProcessingCount: 2,
	}

	if usage.TotalJobs != 50 {
		t.Errorf("TotalJobs = %d, want 50", usage.TotalJobs)
	}
	if usage.CompletedCount+usage.FailedCount+usage.QueuedCount+usage.ProcessingCount != usage.TotalJobs {
		t.Error("Job counts don't add up to TotalJobs")
	}
}

func TestOrphanedFileFields(t *testing.T) {
	orphan := OrphanedFile{
		Path:      "/data/transcodes/hls/999/720p",
		SizeBytes: 1024 * 1024 * 50,
	}

	if orphan.Path != "/data/transcodes/hls/999/720p" {
		t.Errorf("Path = %q, want %q", orphan.Path, "/data/transcodes/hls/999/720p")
	}
	if orphan.SizeBytes != 52428800 {
		t.Errorf("SizeBytes = %d, want 52428800", orphan.SizeBytes)
	}
}

func TestCleanupFilterDefaults(t *testing.T) {
	// Test default filter with all false values
	filter := CleanupFilter{}

	if filter.IncludeCompleted {
		t.Error("Default IncludeCompleted should be false")
	}
	if filter.IncludeFailed {
		t.Error("Default IncludeFailed should be false")
	}
	if filter.IncludeQueued {
		t.Error("Default IncludeQueued should be false")
	}
	if filter.OlderThan != nil {
		t.Error("Default OlderThan should be nil")
	}
	if filter.MediaID != nil {
		t.Error("Default MediaID should be nil")
	}
	if filter.Quality != nil {
		t.Error("Default Quality should be nil")
	}
	if filter.MinSizeBytes != nil {
		t.Error("Default MinSizeBytes should be nil")
	}
	if filter.Limit != nil {
		t.Error("Default Limit should be nil")
	}
	if filter.DryRun {
		t.Error("Default DryRun should be false")
	}
}
