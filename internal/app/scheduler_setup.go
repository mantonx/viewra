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
	"github.com/mantonx/viewra/internal/domain/events"
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

	// Build runtime deps and register all tasks via generated code
	runtimeDeps := buildRuntimeDeps(deps)
	if err := RegisterAllTasks(ctx, svc, runtimeDeps); err != nil {
		return nil, err
	}

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

// buildRuntimeDeps maps app dependencies to scheduler.RuntimeDeps.
func buildRuntimeDeps(deps SchedulerDeps) *appscheduler.RuntimeDeps {
	runtimeDeps := &appscheduler.RuntimeDeps{
		Logger: deps.Logger,
		Config: &appscheduler.RuntimeConfig{
			ScanJobRetentionMinutes: deps.Config.Media.ScanJobRetentionMinutes,
			TranscodeOutputDir:      deps.Config.Media.TranscodeOutputDir,
		},

		// Concrete types passed via any - tasks will type assert
		ScanJobDeleter: deps.Cases.ScanJob,
		LibraryLister:  deps.Cases.Library.Service,
		ImageCleanup:   deps.Cases.Images.Cleanup,
		SessionCleanup: deps.Repos.Session,
	}

	// Wire transcode cleanup if enabled
	if deps.Svcs.CleanupService != nil {
		runtimeDeps.TranscodeCleanup = deps.Svcs.CleanupService
		runtimeDeps.TranscodeRepo = deps.Repos.Transcode
		runtimeDeps.Config.TranscodeCleanup = &appscheduler.TranscodeCleanupConfig{
			Enabled:              deps.Config.Transcode.CleanupEnabled,
			DiskThresholdPercent: deps.Config.Transcode.DiskThresholdPercent,
			DiskWarningPercent:   deps.Config.Transcode.DiskWarningPercent,
			MinFreeSpaceGB:       deps.Config.Transcode.MinFreeSpaceGB,
			MaxAgeHours:          deps.Config.Transcode.MaxAgeDays * 24,
			MaxIdleHours:         deps.Config.Transcode.MaxIdleDays * 24,
			MaxStorageGB:         deps.Config.Transcode.MaxStorageGB,
			CleanupBatchSize:     deps.Config.Transcode.CleanupBatchSize,
			KeepFailedHours:      deps.Config.Transcode.KeepFailedHours,
		}
	}

	return runtimeDeps
}
