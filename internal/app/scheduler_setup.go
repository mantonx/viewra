package app

import (
	"context"
	"database/sql"
	"log/slog"

	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	appscheduler "github.com/mantonx/viewra/internal/application/scheduler"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/domain/events"
	domscheduler "github.com/mantonx/viewra/internal/domain/scheduler"
	schedrepo "github.com/mantonx/viewra/internal/infrastructure/persistence/scheduler"
)

// SchedulerDeps holds dependencies for scheduler initialization.
type SchedulerDeps struct {
	DB       *sql.DB
	DBDriver string
	Config   *appconfig.Config
	Cases    *usecases.UseCases
	Svcs     *services.Services
	Repos    *repositories.Repositories
	Logger   *slog.Logger
}

// InitScheduler creates and configures the new scheduler service.
func InitScheduler(deps SchedulerDeps) (*appscheduler.Service, error) {
	ctx := context.Background()

	// Create repositories
	taskRepo := schedrepo.NewTaskRepository(deps.DB, deps.DBDriver)
	execRepo := schedrepo.NewExecutionRepository(deps.DB, deps.DBDriver)
	lockRepo := schedrepo.NewLockRepository(deps.DB, deps.DBDriver)

	// Create event bus adapter (can be nil)
	var eventBus appscheduler.EventPublisher
	if deps.Svcs.EventBus != nil {
		eventBus = &eventBusAdapter{bus: deps.Svcs.EventBus}
	}

	// Create scheduler service
	svc, err := appscheduler.NewService(
		appscheduler.DefaultConfig(),
		deps.Logger.With("component", "scheduler"),
		taskRepo,
		execRepo,
		lockRepo,
		eventBus,
	)
	if err != nil {
		return nil, err
	}

	// Register internal tasks
	registerInternalTasks(ctx, svc, deps)

	return svc, nil
}

// eventBusAdapter adapts EventBus to EventPublisher interface.
type eventBusAdapter struct {
	bus interface {
		Publish(event events.Event)
	}
}

func (a *eventBusAdapter) Publish(ctx context.Context, event interface{}) {
	// Only publish if event implements events.Event
	if e, ok := event.(events.Event); ok {
		a.bus.Publish(e)
	}
}

// registerInternalTasks registers all built-in scheduled tasks.
func registerInternalTasks(
	ctx context.Context,
	svc *appscheduler.Service,
	deps SchedulerDeps,
) {
	cfg := deps.Config
	cases := deps.Cases
	svcs := deps.Svcs
	repos := deps.Repos
	logger := deps.Logger

	// Task 1: Scan Job Cleanup
	if err := svc.RegisterInternalTask(ctx, domscheduler.InternalTask{
		ID:          "internal:cleanup:scan-jobs",
		Name:        "Scan Job Cleanup",
		Description: "Delete old scan jobs and checkpoints based on retention policy",
		Schedule:    "*/30 * * * *", // Every 30 minutes
		Handler: func(ctx context.Context) error {
			retentionMinutes := cfg.Media.ScanJobRetentionMinutes
			logger.Info("Running scan job cleanup", "retention_minutes", retentionMinutes)

			resp, err := cases.Library.Service.List(ctx)
			if err != nil {
				return err
			}

			for _, lib := range resp.Libraries {
				if err := cases.ScanJob.DeleteOld(ctx, lib.ID, retentionMinutes); err != nil {
					logger.Error("Failed to clean scan jobs for library",
						"library_id", lib.ID,
						"error", err)
				}
			}

			logger.Info("Scan job cleanup completed", "libraries_processed", len(resp.Libraries))
			return nil
		},
		TimeoutSeconds: 300,
	}, "cleanup"); err != nil {
		logger.Error("Failed to register scan job cleanup task", "error", err)
	}

	// Task 2: Image Cache Cleanup
	if err := svc.RegisterInternalTask(ctx, domscheduler.InternalTask{
		ID:          "internal:cleanup:image-cache",
		Name:        "Image Cache Cleanup",
		Description: "Remove orphaned image cache files that are no longer referenced in the database",
		Schedule:    "0 3 * * *", // Daily at 3 AM
		Handler: func(ctx context.Context) error {
			_, err := cases.Images.Cleanup.CleanOrphanedImages(ctx)
			return err
		},
		TimeoutSeconds: 600,
	}, "cleanup"); err != nil {
		logger.Error("Failed to register image cleanup task", "error", err)
	}

	// Task 3: Transcode Policy Cleanup (if enabled)
	if svcs.CleanupService != nil {
		cleanupConfig := cfg.Transcode.ToCleanupSchedulerConfig()

		if err := svc.RegisterInternalTask(ctx, domscheduler.InternalTask{
			ID:          "internal:transcode:policy-cleanup",
			Name:        "Transcode Policy Cleanup",
			Description: "Clean failed/old/idle/orphaned transcodes based on policy rules",
			Schedule:    "0 */6 * * *", // Every 6 hours
			Handler: func(ctx context.Context) error {
				return transcode.PerformPolicyCleanup(ctx, svcs.CleanupService, cleanupConfig)
			},
			TimeoutSeconds: 600,
		}, "transcode"); err != nil {
			logger.Error("Failed to register transcode policy cleanup task", "error", err)
		}

		// Task 4: Transcode Disk Monitor
		if err := svc.RegisterInternalTask(ctx, domscheduler.InternalTask{
			ID:          "internal:transcode:disk-monitor",
			Name:        "Transcode Disk Monitor",
			Description: "Monitor disk usage and perform LRU cleanup if threshold exceeded",
			Schedule:    "*/30 * * * *", // Every 30 minutes
			Handler: func(ctx context.Context) error {
				return transcode.PerformDiskMonitoring(
					ctx,
					svcs.CleanupService,
					svcs.TranscodeRepo,
					cleanupConfig,
					cfg.Media.TranscodeOutputDir,
				)
			},
			TimeoutSeconds: 300,
		}, "transcode"); err != nil {
			logger.Error("Failed to register transcode disk monitor task", "error", err)
		}
	}

	// Task 5: Session Cleanup
	if err := svc.RegisterInternalTask(ctx, domscheduler.InternalTask{
		ID:          "internal:auth:session-cleanup",
		Name:        "Session Cleanup",
		Description: "Remove expired user sessions from the database",
		Schedule:    "0 * * * *", // Every hour
		Handler: func(ctx context.Context) error {
			logger.Info("Running expired session cleanup")
			deleted, err := repos.Session.DeleteExpired(ctx)
			if err != nil {
				logger.Error("Failed to clean expired sessions", "error", err)
				return err
			}
			logger.Info("Expired session cleanup completed", "deleted_count", deleted)
			return nil
		},
		TimeoutSeconds: 120,
	}, "auth"); err != nil {
		logger.Error("Failed to register session cleanup task", "error", err)
	}

	logger.Info("Registered internal scheduled tasks")
}
