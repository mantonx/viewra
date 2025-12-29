package transcode

import (
	"context"

	"github.com/mantonx/viewra/internal/application/scheduler"
	domain "github.com/mantonx/viewra/internal/domain/transcode"
)

// Tasks exports all scheduled tasks for the transcode domain.
// This variable is discovered by the task generator.
var Tasks = []scheduler.TaskBuilder{
	&transcodePolicyCleanupTask{},
	&transcodeDiskMonitorTask{},
}

// transcodePolicyCleanupTask cleans transcodes based on policy rules.
type transcodePolicyCleanupTask struct{}

func (t *transcodePolicyCleanupTask) Definition() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		ID:             "internal:transcode:policy-cleanup",
		Name:           "Transcode Policy Cleanup",
		Description:    "Clean failed/old/idle/orphaned transcodes based on policy rules",
		Schedule:       "0 */6 * * *", // Every 6 hours
		Group:          "transcode",
		TimeoutSeconds: 600,
	}
}

func (t *transcodePolicyCleanupTask) Build(deps *scheduler.RuntimeDeps) func(context.Context) error {
	// Skip if transcode cleanup is not available
	if deps.TranscodeCleanup == nil {
		return nil
	}

	// Type assert to concrete type
	svc := deps.TranscodeCleanup.(*CleanupService)

	return func(ctx context.Context) error {
		config := &CleanupSchedulerConfig{
			KeepFailedHours:  deps.Config.TranscodeCleanup.KeepFailedHours,
			MaxAgeHours:      deps.Config.TranscodeCleanup.MaxAgeHours,
			MaxIdleHours:     deps.Config.TranscodeCleanup.MaxIdleHours,
			CleanupBatchSize: deps.Config.TranscodeCleanup.CleanupBatchSize,
		}
		return PerformPolicyCleanup(ctx, svc, config)
	}
}

// transcodeDiskMonitorTask monitors disk usage and performs LRU cleanup.
type transcodeDiskMonitorTask struct{}

func (t *transcodeDiskMonitorTask) Definition() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		ID:             "internal:transcode:disk-monitor",
		Name:           "Transcode Disk Monitor",
		Description:    "Monitor disk usage and perform LRU cleanup if threshold exceeded",
		Schedule:       "*/30 * * * *", // Every 30 minutes
		Group:          "transcode",
		TimeoutSeconds: 300,
	}
}

func (t *transcodeDiskMonitorTask) Build(deps *scheduler.RuntimeDeps) func(context.Context) error {
	// Skip if transcode cleanup is not available
	if deps.TranscodeCleanup == nil || deps.TranscodeRepo == nil {
		return nil
	}

	// Type assert to concrete types
	svc := deps.TranscodeCleanup.(*CleanupService)

	// Create a repo interface that matches what PerformDiskMonitoring expects
	repo := deps.TranscodeRepo.(interface {
		GetTotalSize(ctx context.Context) (int64, error)
		ListByLRU(ctx context.Context, limit int) ([]*domain.TranscodeJob, error)
	})

	return func(ctx context.Context) error {
		config := &CleanupSchedulerConfig{
			DiskThresholdPercent: deps.Config.TranscodeCleanup.DiskThresholdPercent,
			DiskWarningPercent:   deps.Config.TranscodeCleanup.DiskWarningPercent,
			MinFreeSpaceGB:       deps.Config.TranscodeCleanup.MinFreeSpaceGB,
			MaxStorageGB:         deps.Config.TranscodeCleanup.MaxStorageGB,
			CleanupBatchSize:     deps.Config.TranscodeCleanup.CleanupBatchSize,
		}
		outputDir := deps.Config.TranscodeOutputDir
		return PerformDiskMonitoring(ctx, svc, repo, config, outputDir)
	}
}
