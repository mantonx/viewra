package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// Manager manages scanning operations
type Manager struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager plugins.Manager
	scanners      map[uint]*LibraryScanner // jobID -> scanner mapping
	scannersMu    sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// File monitoring
	fileMonitor   *FileMonitor
	
	// Cleanup service
	cleanupService *CleanupService
}

// NewManager creates a new scanner manager
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create file monitor
	fileMonitor, err := NewFileMonitor(db, eventBus, pluginManager)
	if err != nil {
		logger.Error("Failed to create file monitor", "error", err)
		// Continue without file monitoring
		fileMonitor = nil
	}
	
	return &Manager{
		db:            db,
		eventBus:      eventBus,
		pluginManager: pluginManager,
		scanners:      make(map[uint]*LibraryScanner),
		scannersMu:    sync.RWMutex{},
		ctx:           ctx,
		cancel:        cancel,
		fileMonitor:   fileMonitor,
		cleanupService: NewCleanupService(db),
	}
}

// recoverOrphanedJobs handles scan jobs that were marked as "running" when the backend restarted
// and automatically resumes paused jobs that have made progress
func (m *Manager) recoverOrphanedJobs() error {
	// Find all jobs marked as "running" but not actually running
	var orphanedJobs []database.ScanJob
	if err := m.db.Where("status = ?", "running").Preload("Library").Find(&orphanedJobs).Error; err != nil {
		return fmt.Errorf("failed to query orphaned jobs: %w", err)
	}

	// Find paused jobs that could be auto-resumed
	var pausedJobs []database.ScanJob
	if err := m.db.Where("status = ? AND files_processed > 0", "paused").Preload("Library").Find(&pausedJobs).Error; err != nil {
		fmt.Printf("Warning: Failed to query paused jobs for auto-resume: %v\n", err)
	}

	if len(orphanedJobs) == 0 && len(pausedJobs) == 0 {
		fmt.Println("No orphaned or resumable scan jobs found")
		return nil
	}

	// Handle orphaned running jobs - mark as paused for potential resume
	if len(orphanedJobs) > 0 {
		logger.Info("Found %d orphaned scan jobs from backend restart, initiating recovery...", len(orphanedJobs))

		for _, job := range orphanedJobs {
			recoveryMsg := fmt.Sprintf("Resuming scan from backend restart (had processed %d/%d files)", 
				job.FilesProcessed, job.FilesFound)
			
			// Use status_message for informational recovery message, not error_message
			updates := map[string]interface{}{
				"status": string(utils.StatusPaused),
				"status_message": recoveryMsg,
				"error_message": "", // Clear any previous error message
			}
			
			if err := m.db.Model(&database.ScanJob{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
				logger.Error("Failed to recover orphaned job %d: %v", job.ID, err)
				continue
			}

			logger.Info("✅ Recovered scan job %d for library %d", job.ID, job.LibraryID)
		}

		// Add orphaned jobs to paused jobs for potential auto-resume
		pausedJobs = append(pausedJobs, orphanedJobs...)
	}

	// Auto-resume paused jobs that have made significant progress
	if len(pausedJobs) > 0 {
		logger.Info("Checking %d paused scan jobs for auto-resume eligibility...", len(pausedJobs))

		autoResumedCount := 0
		for _, job := range pausedJobs {
			// Auto-resume jobs that have processed at least 10 files or 1% progress
			shouldAutoResume := job.FilesProcessed >= 10 ||
				(job.FilesFound > 0 && float64(job.FilesProcessed)/float64(job.FilesFound) >= 0.01)

			if shouldAutoResume {
				logger.Info("🔄 Auto-resuming scan job %d (processed %d/%d files, %.1f%% complete)",
					job.ID, job.FilesProcessed, job.FilesFound, job.Progress)

				// Resume the job
				if err := m.ResumeScan(job.ID); err != nil {
					logger.Error("Failed to auto-resume job %d: %v", job.ID, err)
					continue
				}

				// Publish auto-resume event
				if m.eventBus != nil {
					resumeEvent := events.NewSystemEvent(
						events.EventScanResumed,
						"Media Scan Auto-Resumed",
						fmt.Sprintf("Auto-resumed scan job #%d after backend restart", job.ID),
					)
					resumeEvent.Data = map[string]interface{}{
						"libraryId":      job.LibraryID,
						"scanJobId":      job.ID,
						"autoResume":     true,
						"filesProcessed": job.FilesProcessed,
						"progress":       job.Progress,
					}
					m.eventBus.PublishAsync(resumeEvent)
				}

				autoResumedCount++
				logger.Info("✅ Successfully auto-resumed scan job %d", job.ID)
			} else {
				logger.Debug("Scan job %d not eligible for auto-resume (processed %d files, %.1f%% complete)",
					job.ID, job.FilesProcessed, job.Progress)
			}
		}
		
		if autoResumedCount > 0 {
			logger.Info("🎉 Auto-resumed %d scan jobs after backend restart", autoResumedCount)
		} else {
			logger.Info("No paused jobs met auto-resume criteria")
		}
	}

	return nil
}

// StartScan creates and starts a new scan job for the specified library.
// It validates that no scan is already running for the library before starting.
func (m *Manager) StartScan(libraryID uint) (*database.ScanJob, error) {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()

	// Validate that we can start a scan for this library
	if err := utils.ValidateScanJob(m.db, libraryID); err != nil {
		return nil, err
	}

	// Create new scan job in database
	scanJob, err := utils.CreateScanJob(m.db, libraryID)
	if err != nil {
		return nil, err
	}

	// Get library info for the event
	var library database.MediaLibrary
	m.db.First(&library, libraryID)
	libraryPath := library.Path

	// Publish scan start event
	if m.eventBus != nil {
		startEvent := events.NewSystemEvent(
			events.EventScanStarted,
			"Media Scan Started",
			fmt.Sprintf("Starting scan for library #%d at path: %s", libraryID, libraryPath),
		)
		startEvent.Data = map[string]interface{}{
			"libraryId": libraryID,
			"scanJobId": scanJob.ID,
			"path":      libraryPath,
		}
		m.eventBus.PublishAsync(startEvent)
	}

	// Create and register scanner
	scanner := NewLibraryScanner(m.db, scanJob.ID, m.eventBus, m.pluginManager)
	m.scanners[scanJob.ID] = scanner

	// Start scanning in background
	go m.runScanJob(scanner, scanJob.ID, libraryID, false)

	return scanJob, nil
}

// runScanJob executes a scan job in a goroutine and handles cleanup.
func (m *Manager) runScanJob(scanner *LibraryScanner, jobID, libraryID uint, isResume bool) {
	defer func() {
		// Clean up completed or failed scans from active scanners map
		m.removeScanner(jobID)

		// Get final job status
		var currentJob database.ScanJob
		if err := m.db.First(&currentJob, jobID).Error; err == nil {
			// Publish scan completed event
			if m.eventBus != nil && currentJob.Status == "completed" {
				completeEvent := events.NewSystemEvent(
					events.EventScanCompleted,
					"Media Scan Completed",
					fmt.Sprintf("Scan completed for library #%d", libraryID),
				)
				eventData := map[string]interface{}{
					"libraryId":      libraryID,
					"scanJobId":      jobID,
					"filesProcessed": currentJob.FilesProcessed,
					"bytesProcessed": currentJob.BytesProcessed,
				}

				// Only calculate duration if StartedAt is not nil
				if currentJob.StartedAt != nil {
					eventData["duration"] = currentJob.CompletedAt.Sub(*currentJob.StartedAt).String()
				} else {
					eventData["duration"] = "unknown"
				}

				completeEvent.Data = eventData
				m.eventBus.PublishAsync(completeEvent)
				
				// Start file monitoring for completed scans
				if m.fileMonitor != nil {
					if err := m.fileMonitor.StartMonitoring(libraryID, jobID); err != nil {
						logger.Error("Failed to start file monitoring for completed scan", 
							"library_id", libraryID, "job_id", jobID, "error", err)
					} else {
						logger.Info("Started file monitoring for completed scan", 
							"library_id", libraryID, "job_id", jobID)
					}
				}
			}
		}
	}()

	// Start or resume the scanning process
	var err error
	if isResume {
		err = scanner.Resume(libraryID)
	} else {
		err = scanner.Start(libraryID)
	}

	if err != nil {
		// Check if this was a pause request (not an actual error)
		var currentJob database.ScanJob
		if err := m.db.First(&currentJob, jobID).Error; err == nil {
			if currentJob.Status != "paused" {
				// Update job with error status
				utils.UpdateJobStatus(m.db, jobID, utils.StatusFailed, err.Error())

				// Publish scan failed event
				if m.eventBus != nil {
					failEvent := events.NewSystemEvent(
						events.EventScanFailed,
						"Media Scan Failed",
						fmt.Sprintf("Scan failed for library #%d: %v", libraryID, err),
					)
					failEvent.Data = map[string]interface{}{
						"libraryId": libraryID,
						"scanJobId": jobID,
						"error":     err.Error(),
					}
					m.eventBus.PublishAsync(failEvent)
				}
			}
		}
	}
}

// StopScan pauses a running scan job.
// The scan can be resumed later using ResumeScan.
func (m *Manager) StopScan(jobID uint) error {
	m.scannersMu.RLock()
	scanner, exists := m.scanners[jobID]
	m.scannersMu.RUnlock()

	if !exists {
		return m.handleInactiveJobStop(jobID)
	}

	// Get job details for the event
	var scanJob database.ScanJob
	var libraryID uint
	if err := m.db.First(&scanJob, jobID).Error; err == nil {
		libraryID = scanJob.LibraryID
	}

	// Pause the active scanner
	scanner.Pause()

	// Update job status to paused
	err := utils.UpdateJobStatus(m.db, jobID, utils.StatusPaused, "")

	// Publish scan paused event
	if err == nil && m.eventBus != nil {
		pauseEvent := events.NewSystemEvent(
			events.EventScanPaused,
			"Media Scan Paused",
			fmt.Sprintf("Scan paused for library #%d", libraryID),
		)
		pauseEvent.Data = map[string]interface{}{
			"libraryId": libraryID,
			"scanJobId": jobID,
			"progress":  scanJob.Progress,
		}
		m.eventBus.PublishAsync(pauseEvent)
	}

	return err
}

// TerminateScan completely terminates a running scan job and removes it from memory.
// Unlike StopScan which pauses, this permanently stops the scan and cleans up resources.
func (m *Manager) TerminateScan(jobID uint) error {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()

	scanner, exists := m.scanners[jobID]
	if !exists {
		// Job is not active, just update database status
		return m.handleInactiveJobTermination(jobID)
	}

	// Get job details for the event
	var scanJob database.ScanJob
	var libraryID uint
	if err := m.db.First(&scanJob, jobID).Error; err == nil {
		libraryID = scanJob.LibraryID
	}

	// Cancel the active scanner (this calls the context cancel function)
	scanner.Pause()
	
	// Remove from active scanners map immediately
	delete(m.scanners, jobID)

	// Update job status to cancelled
	err := utils.UpdateJobStatus(m.db, jobID, utils.StatusFailed, "Terminated during library deletion")

	// Publish scan terminated event
	if err == nil && m.eventBus != nil {
		terminateEvent := events.NewSystemEvent(
			events.EventScanFailed,
			"Media Scan Terminated",
			fmt.Sprintf("Scan terminated for library #%d", libraryID),
		)
		terminateEvent.Data = map[string]interface{}{
			"libraryId": libraryID,
			"scanJobId": jobID,
			"reason":    "library_deletion",
		}
		m.eventBus.PublishAsync(terminateEvent)
	}

	logger.Info("Scan job terminated", "job_id", jobID, "library_id", libraryID)
	return err
}

// handleInactiveJobStop handles stopping a job that isn't actively running.
func (m *Manager) handleInactiveJobStop(jobID uint) error {
	var scanJob database.ScanJob
	if err := m.db.First(&scanJob, jobID).Error; err != nil {
		return fmt.Errorf("scan job not found: %w", err)
	}

	// Only allow pausing running jobs
	if scanJob.Status == string(utils.StatusRunning) {
		return utils.UpdateJobStatus(m.db, jobID, utils.StatusPaused, "")
	}

	return fmt.Errorf("scan job %d exists but is not running (current status: %s)",
		jobID, scanJob.Status)
}

// handleInactiveJobTermination handles terminating a job that isn't actively running.
func (m *Manager) handleInactiveJobTermination(jobID uint) error {
	var scanJob database.ScanJob
	if err := m.db.First(&scanJob, jobID).Error; err != nil {
		return fmt.Errorf("scan job not found: %w", err)
	}

	// Update status to cancelled/failed
	return utils.UpdateJobStatus(m.db, jobID, utils.StatusFailed, "Terminated during library deletion")
}

// ResumeScan resumes a previously paused scan job.
func (m *Manager) ResumeScan(jobID uint) error {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()

	// Check if scanner is already running
	if _, exists := m.scanners[jobID]; exists {
		return fmt.Errorf("scan job %d is already running", jobID)
	}

	// Load the scan job
	var scanJob database.ScanJob
	if err := m.db.Preload("Library").First(&scanJob, jobID).Error; err != nil {
		return fmt.Errorf("scan job not found: %w", err)
	}

	// Allow resuming paused, pending, or failed jobs (failed jobs are usually from system restarts)
	if scanJob.Status != string(utils.StatusPaused) &&
		scanJob.Status != string(utils.StatusPending) &&
		scanJob.Status != string(utils.StatusFailed) {
		return fmt.Errorf("cannot resume scan job with status: %s", scanJob.Status)
	}

	// Create and register new scanner
	scanner := NewLibraryScanner(m.db, jobID, m.eventBus, m.pluginManager)
	m.scanners[jobID] = scanner

	// Publish scan resumed event
	if m.eventBus != nil {
		resumeEvent := events.NewSystemEvent(
			events.EventScanResumed,
			"Media Scan Resumed",
			fmt.Sprintf("Resumed scan job #%d for library #%d", jobID, scanJob.LibraryID),
		)
		resumeEvent.Data = map[string]interface{}{
			"libraryId": scanJob.LibraryID,
			"scanJobId": jobID,
			"path":      scanJob.Library.Path,
			"progress":  scanJob.Progress,
		}
		m.eventBus.PublishAsync(resumeEvent)
	}

	// Start resumed scanning in background
	go m.runScanJob(scanner, jobID, scanJob.LibraryID, true)

	return nil
}

// GetScanStatus returns the current status of a scan job.
func (m *Manager) GetScanStatus(jobID uint) (*database.ScanJob, error) {
	// Safety checks for nil manager and nil database
	if m == nil {
		return nil, fmt.Errorf("scanner manager is nil")
	}
	if m.db == nil {
		return nil, fmt.Errorf("scanner manager database connection is nil")
	}
	
	var scanJob database.ScanJob
	err := m.db.Preload("Library").First(&scanJob, jobID).Error
	if err != nil {
		return nil, fmt.Errorf("scan job not found: %w", err)
	}

	return &scanJob, nil
}

// GetAllScans returns all scan jobs ordered by creation date.
func (m *Manager) GetAllScans() ([]database.ScanJob, error) {
	var scanJobs []database.ScanJob
	err := m.db.Preload("Library").Order("created_at DESC").Find(&scanJobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get scan jobs: %w", err)
	}

	return scanJobs, nil
}

// GetActiveScanCount returns the number of currently active scans.
func (m *Manager) GetActiveScanCount() int {
	m.scannersMu.RLock()
	defer m.scannersMu.RUnlock()
	return len(m.scanners)
}

// CancelAllScans stops all running scans and marks them as paused.
// This is useful for graceful shutdowns or system restarts.
// Returns the number of scans that were successfully paused.
func (m *Manager) CancelAllScans() (int, error) {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()

	successCount := 0
	for jobID, scanner := range m.scanners {
		// Pause the scanner
		scanner.Pause()

		// Update job status
		if err := utils.UpdateJobStatus(m.db, jobID, utils.StatusPaused, "System shutdown"); err != nil {
			fmt.Printf("Failed to update status for job %d: %v\n", jobID, err)
			continue
		}

		successCount++
	}

	// Note: We don't remove scanners from the map here so they can be resumed later
	fmt.Printf("Successfully paused %d active scan jobs\n", successCount)
	return successCount, nil
}

// CleanupOldJobs removes old completed/failed scan jobs.
// Returns the number of jobs that were cleaned up.
func (m *Manager) CleanupOldJobs() (int64, error) {
	count, err := utils.CleanupOldScanJobs(m.db)
	if err != nil {
		return 0, err
	}

	if count > 0 {
		fmt.Printf("Cleaned up %d old scan jobs\n", count)
	}
	return count, nil
}

// CleanupJobsByLibrary removes all scan jobs associated with a specific library.
// This should be called when a library is deleted to prevent orphaned jobs.
// Returns the number of jobs that were cleaned up.
func (m *Manager) CleanupJobsByLibrary(libraryID uint) (int64, error) {
	// First, identify scanners to stop (without holding the lock)
	var scannersToStop []struct {
		jobID   uint
		scanner *LibraryScanner
	}

	m.scannersMu.RLock()
	for jobID, scanner := range m.scanners {
		// Get the job to check library ID
		var scanJob database.ScanJob
		if err := m.db.First(&scanJob, jobID).Error; err == nil {
			if scanJob.LibraryID == libraryID {
				scannersToStop = append(scannersToStop, struct {
					jobID   uint
					scanner *LibraryScanner
				}{jobID, scanner})
			}
		}
	}
	m.scannersMu.RUnlock()

	// Stop scanners without holding the manager lock (to avoid deadlock)
	var stoppedScanners []uint
	for _, item := range scannersToStop {
		fmt.Printf("Stopping active scanner for job %d (library %d)\n", item.jobID, libraryID)

		// Use a timeout to prevent hanging
		done := make(chan bool, 1)
		go func() {
			item.scanner.Pause()
			done <- true
		}()

		select {
		case <-done:
			fmt.Printf("Successfully paused scanner for job %d\n", item.jobID)
			stoppedScanners = append(stoppedScanners, item.jobID)
		case <-time.After(5 * time.Second):
			fmt.Printf("Timeout pausing scanner for job %d, forcing cleanup\n", item.jobID)
			stoppedScanners = append(stoppedScanners, item.jobID)
		}
	}

	// Now acquire lock to remove scanners from the map
	m.scannersMu.Lock()
	for _, jobID := range stoppedScanners {
		delete(m.scanners, jobID)
	}
	m.scannersMu.Unlock()

	// Delete all scan jobs for this library from the database
	result := m.db.Where("library_id = ?", libraryID).Delete(&database.ScanJob{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup scan jobs for library %d: %w", libraryID, result.Error)
	}

	// NOTE: We do NOT clean up media files and assets when deleting scan jobs
	// Media files should persist even after scan jobs are removed
	// Only the library deletion should trigger comprehensive cleanup

	// Publish cleanup event
	if m.eventBus != nil && result.RowsAffected > 0 {
		cleanupEvent := events.NewSystemEvent(
			events.EventInfo,
			"Scan Jobs Cleaned Up",
			fmt.Sprintf("Cleaned up %d scan jobs for deleted library #%d", result.RowsAffected, libraryID),
		)
		cleanupEvent.Data = map[string]interface{}{
			"libraryId":    libraryID,
			"jobsDeleted":  result.RowsAffected,
			"stoppedScans": len(stoppedScanners),
		}
		m.eventBus.PublishAsync(cleanupEvent)
	}

	fmt.Printf("Cleaned up %d scan jobs for library %d\n", result.RowsAffected, libraryID)
	return result.RowsAffected, nil
}

// CleanupOrphanedJobs removes scan jobs for libraries that no longer exist.
// Returns the number of jobs that were cleaned up.
func (m *Manager) CleanupOrphanedJobs() (int64, error) {
	// Get all existing library IDs
	var existingLibraryIDs []uint
	if err := m.db.Model(&database.MediaLibrary{}).Pluck("id", &existingLibraryIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to get existing library IDs: %w", err)
	}

	// Find scan jobs for non-existent libraries
	var orphanedJobs []database.ScanJob
	query := m.db.Where("library_id NOT IN (?)", existingLibraryIDs)
	if len(existingLibraryIDs) == 0 {
		// If no libraries exist, all scan jobs are orphaned
		query = m.db
	}

	if err := query.Find(&orphanedJobs).Error; err != nil {
		return 0, fmt.Errorf("failed to find orphaned scan jobs: %w", err)
	}

	if len(orphanedJobs) == 0 {
		fmt.Println("No orphaned scan jobs found")
		return 0, nil
	}

	// Identify active scanners to stop (without holding the lock)
	var scannersToStop []struct {
		jobID   uint
		scanner *LibraryScanner
	}

	m.scannersMu.RLock()
	for _, job := range orphanedJobs {
		if scanner, exists := m.scanners[job.ID]; exists {
			scannersToStop = append(scannersToStop, struct {
				jobID   uint
				scanner *LibraryScanner
			}{job.ID, scanner})
		}
	}
	m.scannersMu.RUnlock()

	// Stop scanners without holding the manager lock (to avoid deadlock)
	var stoppedScanners []uint
	for _, item := range scannersToStop {
		// Find the corresponding job for library ID
		var libraryID uint
		for _, job := range orphanedJobs {
			if job.ID == item.jobID {
				libraryID = job.LibraryID
				break
			}
		}
		fmt.Printf("Stopping active scanner for orphaned job %d (library %d)\n", item.jobID, libraryID)

		// Use a timeout to prevent hanging
		done := make(chan bool, 1)
		go func() {
			item.scanner.Pause()
			done <- true
		}()

		select {
		case <-done:
			fmt.Printf("Successfully paused orphaned scanner for job %d\n", item.jobID)
			stoppedScanners = append(stoppedScanners, item.jobID)
		case <-time.After(5 * time.Second):
			fmt.Printf("Timeout pausing orphaned scanner for job %d, forcing cleanup\n", item.jobID)
			stoppedScanners = append(stoppedScanners, item.jobID)
		}
	}

	// Now acquire lock to remove scanners from the map
	m.scannersMu.Lock()
	for _, jobID := range stoppedScanners {
		delete(m.scanners, jobID)
	}
	m.scannersMu.Unlock()

	// Delete orphaned scan jobs from the database
	var orphanedJobIDs []uint
	for _, job := range orphanedJobs {
		orphanedJobIDs = append(orphanedJobIDs, job.ID)
	}

	result := m.db.Where("id IN (?)", orphanedJobIDs).Delete(&database.ScanJob{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete orphaned scan jobs: %w", result.Error)
	}

	// Publish cleanup event
	if m.eventBus != nil && result.RowsAffected > 0 {
		cleanupEvent := events.NewSystemEvent(
			events.EventInfo,
			"Orphaned Scan Jobs Cleaned Up",
			fmt.Sprintf("Cleaned up %d orphaned scan jobs", result.RowsAffected),
		)
		cleanupEvent.Data = map[string]interface{}{
			"jobsDeleted":  result.RowsAffected,
			"stoppedScans": len(stoppedScanners),
		}
		m.eventBus.PublishAsync(cleanupEvent)
	}

	fmt.Printf("Cleaned up %d orphaned scan jobs\n", result.RowsAffected)
	return result.RowsAffected, nil
}

// GetLibraryStats returns comprehensive statistics for a library's scanned files.
func (m *Manager) GetLibraryStats(libraryID uint) (*utils.LibraryStats, error) {
	return utils.GetLibraryStatistics(m.db, libraryID)
}

// removeScanner safely removes a scanner from the active scanners map.
func (m *Manager) removeScanner(jobID uint) {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()
	delete(m.scanners, jobID)
}

// Shutdown gracefully shuts down the manager by pausing all active scans.
func (m *Manager) Shutdown() error {
	fmt.Println("Shutting down scan manager...")
	count, err := m.CancelAllScans()
	if err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}

	// Stop file monitor
	if m.fileMonitor != nil {
		if err := m.fileMonitor.Stop(); err != nil {
			fmt.Printf("Error stopping file monitor: %v\n", err)
		}
	}

	fmt.Printf("Scan manager shutdown complete. Paused %d active scans.\n", count)
	return nil
}

// GetScanProgress returns progress, ETA, and files/sec for a scan job
func (m *Manager) GetScanProgress(jobID uint) (progress float64, eta string, filesPerSec float64, err error) {
	// Safety check for nil manager
	if m == nil {
		return 0, "", 0, fmt.Errorf("scanner manager is nil")
	}
	
	// Safety check for nil mutex
	if m.scanners == nil {
		return 0, "", 0, fmt.Errorf("scanner manager scanners map is nil")
	}
	
	m.scannersMu.RLock()
	scanner, exists := m.scanners[jobID]
	m.scannersMu.RUnlock()

	if !exists {
		return 0, "", 0, fmt.Errorf("no active scanner for job %d", jobID)
	}

	// Safety check for nil scanner
	if scanner == nil {
		return 0, "", 0, fmt.Errorf("scanner for job %d is nil", jobID)
	}

	// Get progress from the scanner's progress estimator
	if scanner.progressEstimator != nil {
		progress, etaTime, filesPerSec := scanner.progressEstimator.GetEstimate()
		eta = etaTime.Format(time.RFC3339)
		return progress, eta, filesPerSec, nil
	}

	return 0, "", 0, fmt.Errorf("no progress available for job %d", jobID)
}

// GetDetailedScanProgress returns detailed scan progress including adaptive worker pool stats
func (m *Manager) GetDetailedScanProgress(jobID uint) (map[string]interface{}, error) {
	// Safety check for nil manager
	if m == nil {
		return nil, fmt.Errorf("scanner manager is nil")
	}
	
	// Safety check for nil mutex - this prevents the nil pointer dereference we've been seeing
	if m.scanners == nil {
		return nil, fmt.Errorf("scanner manager scanners map is nil")
	}
	
	m.scannersMu.RLock()
	scanner, exists := m.scanners[jobID]
	m.scannersMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no active scanner for job %d", jobID)
	}

	// Safety check for nil scanner
	if scanner == nil {
		return nil, fmt.Errorf("scanner for job %d is nil", jobID)
	}

	// Safety check for nil progress estimator
	if scanner.progressEstimator == nil {
		return nil, fmt.Errorf("progress estimator for job %d is nil", jobID)
	}

	// Get basic progress stats
	progress, etaTime, filesPerSec := scanner.progressEstimator.GetEstimate()
	eta := etaTime.Format(time.RFC3339)

	// Get worker pool stats
	activeWorkers, minWorkers, maxWorkers, queueLen := scanner.GetWorkerStats()

	// Get additional stats from progress estimator
	progressStats := scanner.progressEstimator.GetStats()

	// Merge all stats
	result := map[string]interface{}{
		"progress":       progress,
		"eta":            eta,
		"files_per_sec":  filesPerSec,
		"active_workers": activeWorkers,
		"min_workers":    minWorkers,
		"max_workers":    maxWorkers,
		"queue_depth":    queueLen,
	}

	// Add all progress estimator stats
	for k, v := range progressStats {
		result[k] = v
	}

	return result, nil
}

// SetPluginManager updates the plugin manager for this scanner manager
// and all currently running scanners
func (m *Manager) SetPluginManager(pluginMgr plugins.Manager) {
	m.scannersMu.Lock()
	defer m.scannersMu.Unlock()

	m.pluginManager = pluginMgr

	// Update plugin manager in all existing scanners
	for jobID, scanner := range m.scanners {
		scanner.pluginManager = pluginMgr
		fmt.Printf("INFO: Updated plugin manager for scanner job %d\n", jobID)
	}

	fmt.Printf("INFO: Scanner manager plugin manager updated, affected %d active scanners\n", len(m.scanners))
}

// SetParallelMode sets the parallel scanning mode (backward compatibility stub)
// Note: The new scanner implementation always uses parallel scanning for optimal performance
func (m *Manager) SetParallelMode(enabled bool) {
	// This is a stub for backward compatibility
	// The new scanner implementation always uses parallel scanning
	// We could log a deprecation warning here if needed
}

// GetParallelMode returns whether parallel scanning is enabled (backward compatibility stub)
// Note: The new scanner implementation always uses parallel scanning
func (m *Manager) GetParallelMode() bool {
	// Always return true since the new implementation always uses parallel scanning
	return true
}

func (m *Manager) GetScannerManager() *Manager {
	if m == nil {
		logger.Error("Scanner manager is nil")
		return nil
	}

	if m.db == nil {
		logger.Error("Scanner manager database connection is nil")
		return nil
	}

	return m
}

// RecoverOrphanedJobs exposes the orphaned job recovery functionality for public use
func (m *Manager) RecoverOrphanedJobs() error {
	return m.recoverOrphanedJobs()
}

// StartFileMonitoring starts the file monitoring service
func (m *Manager) StartFileMonitoring() error {
	if m.fileMonitor == nil {
		return fmt.Errorf("file monitor not available")
	}
	return m.fileMonitor.Start()
}

// GetMonitoringStatus returns the current monitoring status for libraries
func (m *Manager) GetMonitoringStatus() map[uint]*MonitoredLibrary {
	if m.fileMonitor == nil {
		return make(map[uint]*MonitoredLibrary)
	}
	return m.fileMonitor.GetMonitoringStatus()
}

// CleanupOrphanedAssets removes assets that reference non-existent media files
func (m *Manager) CleanupOrphanedAssets() (int, int, error) {
	if m.cleanupService == nil {
		return 0, 0, fmt.Errorf("cleanup service not available")
	}
	return m.cleanupService.CleanupOrphanedAssets()
}

// CleanupOrphanedFiles removes asset files from disk that have no corresponding database records
func (m *Manager) CleanupOrphanedFiles() (int, error) {
	if m.cleanupService == nil {
		return 0, fmt.Errorf("cleanup service not available")
	}
	return m.cleanupService.CleanupOrphanedFiles()
}

// CleanupScanJob removes a scan job and all its discovered files and assets
func (m *Manager) CleanupScanJob(scanJobID uint) error {
	if m.cleanupService == nil {
		return fmt.Errorf("cleanup service not available")
	}
	
	// Clean up all data discovered by this scan job
	if err := m.cleanupService.CleanupScanJobData(scanJobID); err != nil {
		return fmt.Errorf("failed to cleanup scan job data: %w", err)
	}
	
	// Remove the scan job record itself
	result := m.db.Delete(&database.ScanJob{}, scanJobID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete scan job record: %w", result.Error)
	}
	
	logger.Info("Scan job completely removed", "scan_job_id", scanJobID)
	return nil
}
