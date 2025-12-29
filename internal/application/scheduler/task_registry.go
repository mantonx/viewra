package scheduler

import (
	"context"
	"log/slog"
)

// TaskDefinition describes a scheduled task's metadata.
type TaskDefinition struct {
	ID             string
	Name           string
	Description    string
	Schedule       string // Cron expression
	Group          string // For UI grouping (e.g., "cleanup", "transcode", "auth")
	TimeoutSeconds int    // 0 means use default (300s)
}

// TaskBuilder creates task handlers with access to runtime dependencies.
// Each domain implements this interface for its scheduled tasks.
type TaskBuilder interface {
	// Definition returns the task's metadata.
	Definition() TaskDefinition

	// Build creates the task handler using runtime dependencies.
	// Returns nil if the task should not be registered (e.g., optional feature disabled).
	Build(deps *RuntimeDeps) func(context.Context) error
}

// RuntimeDeps provides all dependencies that scheduled tasks might need.
// Uses concrete types via any to avoid interface mismatch issues.
// Tasks should type-assert to the expected concrete types.
type RuntimeDeps struct {
	Logger *slog.Logger
	Config *RuntimeConfig

	// ScanJobDeleter: *scanjob.Service - implements DeleteOld method
	ScanJobDeleter any

	// LibraryLister: *library.LibraryService - implements List method
	LibraryLister any

	// ImageCleanup: *images.CleanupUseCase - implements CleanOrphanedImages method
	ImageCleanup any

	// TranscodeCleanup: *transcode.CleanupService (can be nil if transcode disabled)
	TranscodeCleanup any

	// TranscodeRepo: *transcode.Repository (can be nil if transcode disabled)
	TranscodeRepo any

	// SessionCleanup: *user.SessionRepository - implements DeleteExpired method
	SessionCleanup any
}

// RuntimeConfig holds configuration values that tasks need.
type RuntimeConfig struct {
	ScanJobRetentionMinutes int
	TranscodeOutputDir      string
	TranscodeCleanup        *TranscodeCleanupConfig
}

// TranscodeCleanupConfig mirrors the config needed for transcode cleanup tasks.
type TranscodeCleanupConfig struct {
	Enabled              bool
	DiskThresholdPercent int
	DiskWarningPercent   int
	MinFreeSpaceGB       int64
	MaxAgeHours          int
	MaxIdleHours         int
	MaxStorageGB         int64
	CleanupBatchSize     int
	KeepFailedHours      int
}
