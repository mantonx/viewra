package scanjob

import (
	"context"

	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/application/scheduler"
)

// Tasks exports all scheduled tasks for the scanjob domain.
// This variable is discovered by the task generator.
var Tasks = []scheduler.TaskBuilder{
	&scanJobCleanupTask{},
}

// scanJobCleanupTask cleans old scan jobs based on retention policy.
type scanJobCleanupTask struct{}

func (t *scanJobCleanupTask) Definition() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		ID:             "internal:cleanup:scan-jobs",
		Name:           "Scan Job Cleanup",
		Description:    "Delete old scan jobs and checkpoints based on retention policy",
		Schedule:       "*/30 * * * *", // Every 30 minutes
		Group:          "cleanup",
		TimeoutSeconds: 300,
	}
}

func (t *scanJobCleanupTask) Build(deps *scheduler.RuntimeDeps) func(context.Context) error {
	// Type assert the dependencies
	scanJobSvc := deps.ScanJobDeleter.(*Service)
	librarySvc := deps.LibraryLister.(*library.LibraryService)

	return func(ctx context.Context) error {
		deps.Logger.Info("Running scan job cleanup", "retention_minutes", deps.Config.ScanJobRetentionMinutes)

		// Get library IDs
		resp, err := librarySvc.List(ctx)
		if err != nil {
			return err
		}

		for _, lib := range resp.Libraries {
			if err := scanJobSvc.DeleteOld(ctx, lib.ID, deps.Config.ScanJobRetentionMinutes); err != nil {
				deps.Logger.Error("Failed to clean scan jobs for library",
					"library_id", lib.ID,
					"error", err)
			}
		}

		deps.Logger.Info("Scan job cleanup completed", "libraries_processed", len(resp.Libraries))
		return nil
	}
}
