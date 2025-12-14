package enrichment

import (
	"testing"
	"time"
)

func TestMediaType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		mt       MediaType
		expected string
	}{
		{"movie", MediaTypeMovie, "movie"},
		{"tv", MediaTypeTV, "tv"},
		{"music", MediaTypeMusic, "music"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mt) != tt.expected {
				t.Errorf("MediaType = %q, want %q", tt.mt, tt.expected)
			}
		})
	}
}

func TestJobStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected string
	}{
		{"pending", JobStatusPending, "pending"},
		{"processing", JobStatusProcessing, "processing"},
		{"completed", JobStatusCompleted, "completed"},
		{"failed", JobStatusFailed, "failed"},
		{"skipped", JobStatusSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("JobStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestErrorCategory_Constants(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{"network", ErrorCategoryNetwork, "network"},
		{"rate_limit", ErrorCategoryRateLimit, "rate_limit"},
		{"not_found", ErrorCategoryNotFound, "not_found"},
		{"parsing", ErrorCategoryParsing, "parsing"},
		{"plugin", ErrorCategoryPlugin, "plugin"},
		{"database", ErrorCategoryDatabase, "database"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.category) != tt.expected {
				t.Errorf("ErrorCategory = %q, want %q", tt.category, tt.expected)
			}
		})
	}
}

func TestQueueJob_ShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		maxAttempts int
		expected    bool
	}{
		{"zero attempts, max 3", 0, 3, true},
		{"1 attempt, max 3", 1, 3, true},
		{"2 attempts, max 3", 2, 3, true},
		{"3 attempts, max 3 - no more retries", 3, 3, false},
		{"4 attempts, max 3 - exceeded", 4, 3, false},
		{"zero attempts, max 0 - no retries configured", 0, 0, false},
		{"1 attempt, max 1 - exhausted", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &QueueJob{
				Attempts:    tt.attempts,
				MaxAttempts: tt.maxAttempts,
			}
			if got := job.ShouldRetry(); got != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCategory_IsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected bool
	}{
		{"network errors are retryable", ErrorCategoryNetwork, true},
		{"rate limit errors are retryable", ErrorCategoryRateLimit, true},
		{"not found errors are not retryable", ErrorCategoryNotFound, false},
		{"parsing errors are not retryable", ErrorCategoryParsing, false},
		{"plugin errors are not retryable", ErrorCategoryPlugin, false},
		{"database errors are not retryable", ErrorCategoryDatabase, false},
		{"empty category is not retryable", ErrorCategory(""), false},
		{"unknown category is not retryable", ErrorCategory("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestQueueJob_Fields(t *testing.T) {
	now := time.Now()
	retryAt := now.Add(30 * time.Second)

	job := &QueueJob{
		ID:            123,
		MediaID:       456,
		Stage:         "tmdb",
		Priority:      5,
		Status:        JobStatusPending,
		Attempts:      1,
		MaxAttempts:   3,
		ErrorMessage:  "connection timeout",
		ErrorCategory: ErrorCategoryNetwork,
		NextRetryAt:   &retryAt,
		LockedBy:      "worker-1",
		LockedAt:      &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if job.ID != 123 {
		t.Errorf("ID = %d, want 123", job.ID)
	}
	if job.MediaID != 456 {
		t.Errorf("MediaID = %d, want 456", job.MediaID)
	}
	if job.Stage != "tmdb" {
		t.Errorf("Stage = %s, want tmdb", job.Stage)
	}
	if job.Priority != 5 {
		t.Errorf("Priority = %d, want 5", job.Priority)
	}
	if job.Status != JobStatusPending {
		t.Errorf("Status = %s, want pending", job.Status)
	}
	if job.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", job.Attempts)
	}
	if job.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", job.MaxAttempts)
	}
	if job.ErrorMessage != "connection timeout" {
		t.Errorf("ErrorMessage = %s, want 'connection timeout'", job.ErrorMessage)
	}
	if job.ErrorCategory != ErrorCategoryNetwork {
		t.Errorf("ErrorCategory = %s, want network", job.ErrorCategory)
	}
	if job.NextRetryAt == nil || !job.NextRetryAt.Equal(retryAt) {
		t.Errorf("NextRetryAt = %v, want %v", job.NextRetryAt, retryAt)
	}
	if job.LockedBy != "worker-1" {
		t.Errorf("LockedBy = %s, want worker-1", job.LockedBy)
	}
	if job.LockedAt == nil || !job.LockedAt.Equal(now) {
		t.Errorf("LockedAt = %v, want %v", job.LockedAt, now)
	}
}

func TestQueueStats_Fields(t *testing.T) {
	stats := &QueueStats{
		Stage:           "nfo",
		PendingCount:    100,
		ProcessingCount: 5,
		CompletedCount:  800,
		FailedCount:     10,
		SkippedCount:    85,
		TotalCount:      1000,
	}

	if stats.Stage != "nfo" {
		t.Errorf("Stage = %s, want nfo", stats.Stage)
	}
	if stats.PendingCount != 100 {
		t.Errorf("PendingCount = %d, want 100", stats.PendingCount)
	}
	if stats.ProcessingCount != 5 {
		t.Errorf("ProcessingCount = %d, want 5", stats.ProcessingCount)
	}
	if stats.CompletedCount != 800 {
		t.Errorf("CompletedCount = %d, want 800", stats.CompletedCount)
	}
	if stats.FailedCount != 10 {
		t.Errorf("FailedCount = %d, want 10", stats.FailedCount)
	}
	if stats.SkippedCount != 85 {
		t.Errorf("SkippedCount = %d, want 85", stats.SkippedCount)
	}
	if stats.TotalCount != 1000 {
		t.Errorf("TotalCount = %d, want 1000", stats.TotalCount)
	}
}

func TestStatus_Fields(t *testing.T) {
	now := time.Now()
	status := &Status{
		MediaID:      42,
		Stage:        "tmdb",
		Status:       JobStatusCompleted,
		PluginID:     "tmdb-enricher",
		CompletedAt:  &now,
		ErrorMessage: "",
		MetadataJSON: `{"title": "The Matrix"}`,
	}

	if status.MediaID != 42 {
		t.Errorf("MediaID = %d, want 42", status.MediaID)
	}
	if status.Stage != "tmdb" {
		t.Errorf("Stage = %s, want tmdb", status.Stage)
	}
	if status.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want completed", status.Status)
	}
	if status.PluginID != "tmdb-enricher" {
		t.Errorf("PluginID = %s, want tmdb-enricher", status.PluginID)
	}
	if status.CompletedAt == nil || !status.CompletedAt.Equal(now) {
		t.Errorf("CompletedAt = %v, want %v", status.CompletedAt, now)
	}
	if status.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %s, want empty", status.ErrorMessage)
	}
	if status.MetadataJSON != `{"title": "The Matrix"}` {
		t.Errorf("MetadataJSON = %s, want JSON", status.MetadataJSON)
	}
}

func TestPipelineStage_Fields(t *testing.T) {
	now := time.Now()
	stage := &PipelineStage{
		ID:         1,
		MediaType:  MediaTypeMovie,
		PluginID:   "nfo",
		StageName:  "nfo",
		Position:   0,
		Enabled:    true,
		ConfigJSON: `{"timeout": 60}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if stage.ID != 1 {
		t.Errorf("ID = %d, want 1", stage.ID)
	}
	if stage.MediaType != MediaTypeMovie {
		t.Errorf("MediaType = %s, want movie", stage.MediaType)
	}
	if stage.PluginID != "nfo" {
		t.Errorf("PluginID = %s, want nfo", stage.PluginID)
	}
	if stage.StageName != "nfo" {
		t.Errorf("StageName = %s, want nfo", stage.StageName)
	}
	if stage.Position != 0 {
		t.Errorf("Position = %d, want 0", stage.Position)
	}
	if !stage.Enabled {
		t.Errorf("Enabled = %v, want true", stage.Enabled)
	}
	if stage.ConfigJSON != `{"timeout": 60}` {
		t.Errorf("ConfigJSON = %s, want JSON", stage.ConfigJSON)
	}
}

func TestExternalID_Fields(t *testing.T) {
	now := time.Now()
	extID := &ExternalID{
		MediaID:    42,
		Provider:   "imdb",
		ExternalID: "tt0133093",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if extID.MediaID != 42 {
		t.Errorf("MediaID = %d, want 42", extID.MediaID)
	}
	if extID.Provider != "imdb" {
		t.Errorf("Provider = %s, want imdb", extID.Provider)
	}
	if extID.ExternalID != "tt0133093" {
		t.Errorf("ExternalID = %s, want tt0133093", extID.ExternalID)
	}
}

func TestMetadataSource_Fields(t *testing.T) {
	now := time.Now()
	source := &MetadataSource{
		MediaID:   42,
		FieldName: "title",
		PluginID:  "nfo",
		RawValue:  "The Matrix",
		UpdatedAt: now,
	}

	if source.MediaID != 42 {
		t.Errorf("MediaID = %d, want 42", source.MediaID)
	}
	if source.FieldName != "title" {
		t.Errorf("FieldName = %s, want title", source.FieldName)
	}
	if source.PluginID != "nfo" {
		t.Errorf("PluginID = %s, want nfo", source.PluginID)
	}
	if source.RawValue != "The Matrix" {
		t.Errorf("RawValue = %s, want 'The Matrix'", source.RawValue)
	}
}
