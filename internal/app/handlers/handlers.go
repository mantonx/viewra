package handlers

import (
	"database/sql"
	"log/slog"

	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	"github.com/mantonx/viewra/internal/infrastructure/scheduler"
)

// Handlers holds all HTTP handlers grouped by domain
type Handlers struct {
	Health    *handlers.HealthHandler
	Browser   *handlers.BrowserHandler
	ScanJob   *handlers.ScanJobHandler
	Progress  *handlers.ProgressHandler
	Transcode *handlers.TranscodeHandler
	Images    *handlers.ImagesHandler
	Scheduler *handlers.SchedulerHandler
}

// BuildHandlers creates all HTTP handler instances
func BuildHandlers(
	db *sql.DB,
	repos *repositories.Repositories,
	svcs *services.Services,
	cases *usecases.UseCases,
	taskScheduler *scheduler.Scheduler,
	transcodeOutputDir string,
	logger *slog.Logger,
) *Handlers {
	// Create basic handlers
	healthHandler := handlers.NewHealthHandler(db, taskScheduler, svcs.TranscodeQueue)
	browserHandler := handlers.NewBrowserHandler(svcs.PathBrowser)
	scanJobHandler := handlers.NewScanJobHandler(repos.ScanJob, repos.Checkpoint, repos.ScanState, cases.Library.Scan, logger)
	progressHandler := handlers.NewProgressHandler(repos.Progress)

	// Create image handler
	imagesHandler := handlers.NewImagesHandler(
		cases.Images.Get,
		cases.Images.GetMedia,
		cases.Images.GetEntity,
		cases.Images.GetBatch,
		svcs.ImageTransformer,
		svcs.ImageCache,
	)

	// Create transcode handler (if transcode is enabled)
	var transcodeHandler *handlers.TranscodeHandler
	if svcs.TranscodeQueue != nil && cases.Transcode.CreateJob != nil {
		transcodeHandler = handlers.NewTranscodeHandler(
			cases.Transcode.CreateJob,
			cases.Transcode.GetStatus,
			cases.Transcode.ServeManifest,
			svcs.TranscodeQueue,
			svcs.CleanupService,
			svcs.SessionManager,
			repos.Media,
			transcodeOutputDir,
		)
	}

	// Create scheduler handler
	schedulerHandler := handlers.NewSchedulerHandler(taskScheduler)

	return &Handlers{
		Health:    healthHandler,
		Browser:   browserHandler,
		ScanJob:   scanJobHandler,
		Progress:  progressHandler,
		Transcode: transcodeHandler,
		Images:    imagesHandler,
		Scheduler: schedulerHandler,
	}
}
