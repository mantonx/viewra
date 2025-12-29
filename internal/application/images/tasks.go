package images

import (
	"context"

	"github.com/mantonx/viewra/internal/application/scheduler"
)

// Tasks exports all scheduled tasks for the images domain.
// This variable is discovered by the task generator.
var Tasks = []scheduler.TaskBuilder{
	&imageCacheCleanupTask{},
}

// imageCacheCleanupTask removes orphaned image cache files.
type imageCacheCleanupTask struct{}

func (t *imageCacheCleanupTask) Definition() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		ID:             "internal:cleanup:image-cache",
		Name:           "Image Cache Cleanup",
		Description:    "Remove orphaned image cache files that are no longer referenced in the database",
		Schedule:       "0 3 * * *", // Daily at 3 AM
		Group:          "cleanup",
		TimeoutSeconds: 600,
	}
}

func (t *imageCacheCleanupTask) Build(deps *scheduler.RuntimeDeps) func(context.Context) error {
	// Type assert the dependency
	cleanupUseCase := deps.ImageCleanup.(*CleanupUseCase)

	return func(ctx context.Context) error {
		stats, err := cleanupUseCase.CleanOrphanedImages(ctx)
		if err != nil {
			return err
		}
		deleted := 0
		if stats != nil {
			deleted = stats.DeletedFiles
		}
		deps.Logger.Info("Image cache cleanup completed", "deleted_count", deleted)
		return nil
	}
}
