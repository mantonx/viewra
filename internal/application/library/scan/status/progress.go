package status

import (
	"context"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// ProgressUpdater defines the interface for updating scan progress.
// This allows the status package to update progress without knowing
// the concrete implementation of the scan job repository.
type ProgressUpdater interface {
	UpdateProgress(ctx context.Context, jobID int64, progress *scanner.Progress) error
}

// ProgressUpdate contains the fields needed to update scan progress.
// Use the builder methods to set values, then call Update() to persist.
type ProgressUpdate struct {
	updater        ProgressUpdater
	logger         *slog.Logger
	jobID          int64
	phase          scanner.ScanPhase
	filesFound     int64
	filesProcessed int64
	errorCount     int64
	warningCount   int64
	estimatedTotal int64
	discoveryDone  bool

	// Optional event publishing
	eventBus  *events.Bus
	libraryID int64
}

// NewProgressUpdate creates a progress update builder for the given job.
func NewProgressUpdate(updater ProgressUpdater, logger *slog.Logger, jobID int64) *ProgressUpdate {
	return &ProgressUpdate{
		updater: updater,
		logger:  logger,
		jobID:   jobID,
	}
}

// Phase sets the current scan phase.
func (p *ProgressUpdate) Phase(phase scanner.ScanPhase) *ProgressUpdate {
	p.phase = phase
	return p
}

// FilesFound sets the number of files discovered.
func (p *ProgressUpdate) FilesFound(count int64) *ProgressUpdate {
	p.filesFound = count
	return p
}

// FilesProcessed sets the number of files processed.
func (p *ProgressUpdate) FilesProcessed(count int64) *ProgressUpdate {
	p.filesProcessed = count
	return p
}

// Errors sets the error count.
func (p *ProgressUpdate) Errors(count int64) *ProgressUpdate {
	p.errorCount = count
	return p
}

// Warnings sets the warning count.
func (p *ProgressUpdate) Warnings(count int64) *ProgressUpdate {
	p.warningCount = count
	return p
}

// EstimatedTotal sets the estimated total file count (from previous scan).
func (p *ProgressUpdate) EstimatedTotal(count int64) *ProgressUpdate {
	p.estimatedTotal = count
	return p
}

// DiscoveryDone marks discovery as complete.
func (p *ProgressUpdate) DiscoveryDone() *ProgressUpdate {
	p.discoveryDone = true
	return p
}

// WithEventBus enables event publishing for this progress update.
func (p *ProgressUpdate) WithEventBus(bus *events.Bus) *ProgressUpdate {
	p.eventBus = bus
	return p
}

// WithLibraryID sets the library ID for event publishing.
func (p *ProgressUpdate) WithLibraryID(libraryID int64) *ProgressUpdate {
	p.libraryID = libraryID
	return p
}

// FromCheckpointStats populates progress from checkpoint statistics.
func (p *ProgressUpdate) FromCheckpointStats(stats *scanner.CheckpointStats) *ProgressUpdate {
	if stats != nil {
		p.filesProcessed = stats.ProcessedFiles
		p.errorCount = stats.FailedFiles
		p.warningCount = stats.WarningFiles
	}
	return p
}

// FromJob copies relevant fields from an existing scan job.
func (p *ProgressUpdate) FromJob(job *scanner.ScanJob) *ProgressUpdate {
	if job != nil {
		p.filesFound = job.FilesFound
		p.phase = job.Phase
		p.estimatedTotal = job.EstimatedTotal
		p.discoveryDone = job.DiscoveryDone
	}
	return p
}

// Update persists the progress to the database and publishes a progress event.
// Returns any error from the repository.
func (p *ProgressUpdate) Update(ctx context.Context) error {
	progress := &scanner.Progress{
		FilesFound:     p.filesFound,
		FilesProcessed: p.filesProcessed,
		ErrorCount:     p.errorCount,
		WarningCount:   p.warningCount,
		LastUpdate:     time.Now(),
		Phase:          p.phase,
		EstimatedTotal: p.estimatedTotal,
		DiscoveryDone:  p.discoveryDone,
	}

	err := p.updater.UpdateProgress(ctx, p.jobID, progress)

	// Publish progress event if event bus is configured
	p.publishProgressEvent()

	return err
}

// publishProgressEvent publishes a scan.progress event if EventBus is configured.
func (p *ProgressUpdate) publishProgressEvent() {
	if p.eventBus == nil {
		return
	}

	// Calculate progress percentage
	var progressPercent float64
	if p.filesFound > 0 {
		progressPercent = float64(p.filesProcessed) / float64(p.filesFound) * 100
	}

	p.eventBus.Publish(events.NewEvent(events.EventScanProgress, "scanner").
		WithLibraryID(p.libraryID).
		WithData("job_id", p.jobID).
		WithData("status", "running").
		WithData("phase", string(p.phase)).
		WithData("progress", progressPercent).
		WithData("files_found", p.filesFound).
		WithData("files_processed", p.filesProcessed).
		WithData("error_count", p.errorCount).
		WithData("warning_count", p.warningCount).
		WithData("estimated_total", p.estimatedTotal).
		WithData("discovery_done", p.discoveryDone).
		Build())
}

// UpdateAsync persists the progress in a background goroutine.
// Logs a warning if the update fails.
func (p *ProgressUpdate) UpdateAsync(ctx context.Context) {
	go func() {
		if err := p.Update(ctx); err != nil {
			p.logger.Warn("Failed to update progress",
				"job_id", p.jobID,
				"phase", p.phase,
				"error", err)
		}
	}()
}
