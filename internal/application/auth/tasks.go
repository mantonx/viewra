package auth

import (
	"context"

	"github.com/mantonx/viewra/internal/application/scheduler"
)

// sessionCleaner interface matches the SessionRepository.DeleteExpired method.
type sessionCleaner interface {
	DeleteExpired(ctx context.Context) (int64, error)
}

// Tasks exports all scheduled tasks for the auth domain.
// This variable is discovered by the task generator.
var Tasks = []scheduler.TaskBuilder{
	&sessionCleanupTask{},
}

// sessionCleanupTask removes expired user sessions.
type sessionCleanupTask struct{}

func (t *sessionCleanupTask) Definition() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		ID:             "internal:auth:session-cleanup",
		Name:           "Session Cleanup",
		Description:    "Remove expired user sessions from the database",
		Schedule:       "0 * * * *", // Every hour
		Group:          "auth",
		TimeoutSeconds: 120,
	}
}

func (t *sessionCleanupTask) Build(deps *scheduler.RuntimeDeps) func(context.Context) error {
	// Type assert to the interface (matches SessionRepository)
	sessionRepo := deps.SessionCleanup.(sessionCleaner)

	return func(ctx context.Context) error {
		deps.Logger.Info("Running expired session cleanup")
		deleted, err := sessionRepo.DeleteExpired(ctx)
		if err != nil {
			deps.Logger.Error("Failed to clean expired sessions", "error", err)
			return err
		}
		deps.Logger.Info("Expired session cleanup completed", "deleted_count", deleted)
		return nil
	}
}
