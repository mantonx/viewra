package scanner

import (
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// Manager handles multiple concurrent file scanning operations.
// It manages the lifecycle of scan jobs and provides a centralized
// interface for starting, stopping, and monitoring scan operations.
type Manager struct {
	db            *gorm.DB
	scanners      map[uint]*ParallelFileScanner // jobID -> scanner mapping
	mu            sync.RWMutex                  // protects scanners map
	eventBus      events.EventBus               // system event bus for notifications
	pluginManager plugins.Manager               // plugin manager for scanner hooks
	parallelMode  bool                          // whether parallel scanning is enabled
}

// NewManager creates a new scanner manager instance.
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager) *Manager {
	manager := &Manager{
		db:            db,
		scanners:      make(map[uint]*ParallelFileScanner),
		eventBus:      eventBus,
		pluginManager: pluginManager,
	}
	
	// Recover any orphaned scan jobs from previous sessions
	if err := manager.recoverOrphanedJobs(); err != nil {
		fmt.Printf("Warning: Failed to recover orphaned scan jobs: %v\n", err)
	}
	
	return manager
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
		fmt.Printf("Found %d orphaned scan jobs, marking as paused for potential resume\n", len(orphanedJobs))
		
		for _, job := range orphanedJobs {
			errorMsg := "Scanner process terminated during backend restart - marked for resume"
			if err := utils.UpdateJobStatus(m.db, job.ID, utils.StatusPaused, errorMsg); err != nil {
				fmt.Printf("Failed to update orphaned job %d: %v\n", job.ID, err)
				continue
			}
			
			fmt.Printf("Marked orphaned scan job %d as paused\n", job.ID)
		}
		
		// Add orphaned jobs to paused jobs for potential auto-resume
		pausedJobs = append(pausedJobs, orphanedJobs...)
	}
	
	// Auto-resume paused jobs that have made significant progress
	if len(pausedJobs) > 0 {
		fmt.Printf("Found %d paused scan jobs, checking for auto-resume candidates\n", len(pausedJobs))
		
		for _, job := range pausedJobs {
			// Auto-resume jobs that have processed at least 10 files or 1% progress
			shouldAutoResume := job.FilesProcessed >= 10 || 
				(job.FilesFound > 0 && float64(job.FilesProcessed)/float64(job.FilesFound) >= 0.01)
			
			if shouldAutoResume {
				fmt.Printf("Auto-resuming scan job %d (processed %d/%d files)\n", 
					job.ID, job.FilesProcessed, job.FilesFound)
				
				// Resume the job
				if err := m.ResumeScan(job.ID); err != nil {
					fmt.Printf("Failed to auto-resume job %d: %v\n", job.ID, err)
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
				
				fmt.Printf("Successfully auto-resumed scan job %d\n", job.ID)
			} else {
				fmt.Printf("Scan job %d not eligible for auto-resume (processed %d files)\n", 
					job.ID, job.FilesProcessed)
			}
		}
	}
	
	return nil
}

// StartScan creates and starts a new scan job for the specified library.
// It validates that no scan is already running for the library before starting.
func (m *Manager) StartScan(libraryID uint) (*database.ScanJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	scanner := NewParallelFileScanner(m.db, scanJob.ID, m.eventBus, m.pluginManager)
	m.scanners[scanJob.ID] = scanner
	
	// Start scanning in background
	go m.runScanJob(scanner, scanJob.ID, libraryID, false)

	return scanJob, nil
}

// runScanJob executes a scan job in a goroutine and handles cleanup.
func (m *Manager) runScanJob(scanner *ParallelFileScanner, jobID, libraryID uint, isResume bool) {
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
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

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

// ResumeScan resumes a previously paused scan job.
func (m *Manager) ResumeScan(jobID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	scanner := NewParallelFileScanner(m.db, jobID, m.eventBus, m.pluginManager)
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.scanners)
}

// CancelAllScans stops all running scans and marks them as paused.
// This is useful for graceful shutdowns or system restarts.
// Returns the number of scans that were successfully paused.
func (m *Manager) CancelAllScans() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
		scanner *ParallelFileScanner
	}
	
	m.mu.RLock()
	for jobID, scanner := range m.scanners {
		// Get the job to check library ID
		var scanJob database.ScanJob
		if err := m.db.First(&scanJob, jobID).Error; err == nil {
			if scanJob.LibraryID == libraryID {
				scannersToStop = append(scannersToStop, struct {
					jobID   uint
					scanner *ParallelFileScanner
				}{jobID, scanner})
			}
		}
	}
	m.mu.RUnlock()

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
	m.mu.Lock()
	for _, jobID := range stoppedScanners {
		delete(m.scanners, jobID)
	}
	m.mu.Unlock()

	// Delete all scan jobs for this library from the database
	result := m.db.Where("library_id = ?", libraryID).Delete(&database.ScanJob{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup scan jobs for library %d: %w", libraryID, result.Error)
	}

	// Publish cleanup event
	if m.eventBus != nil && result.RowsAffected > 0 {
		cleanupEvent := events.NewSystemEvent(
			events.EventInfo,
			"Scan Jobs Cleaned Up",
			fmt.Sprintf("Cleaned up %d scan jobs for deleted library #%d", result.RowsAffected, libraryID),
		)
		cleanupEvent.Data = map[string]interface{}{
			"libraryId":     libraryID,
			"jobsDeleted":   result.RowsAffected,
			"stoppedScans":  len(stoppedScanners),
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
		scanner *ParallelFileScanner
	}
	
	m.mu.RLock()
	for _, job := range orphanedJobs {
		if scanner, exists := m.scanners[job.ID]; exists {
			scannersToStop = append(scannersToStop, struct {
				jobID   uint
				scanner *ParallelFileScanner
			}{job.ID, scanner})
		}
	}
	m.mu.RUnlock()

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
	m.mu.Lock()
	for _, jobID := range stoppedScanners {
		delete(m.scanners, jobID)
	}
	m.mu.Unlock()

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
			"jobsDeleted":   result.RowsAffected,
			"stoppedScans":  len(stoppedScanners),
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
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scanners, jobID)
}

// SetParallelMode enables or disables parallel scanning mode.
func (m *Manager) SetParallelMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parallelMode = enabled
}

// GetParallelMode returns whether parallel scanning is currently enabled.
func (m *Manager) GetParallelMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.parallelMode
}

// Shutdown gracefully shuts down the manager by pausing all active scans.
func (m *Manager) Shutdown() error {
	fmt.Println("Shutting down scan manager...")
	count, err := m.CancelAllScans()
	if err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}
	
	fmt.Printf("Scan manager shutdown complete. Paused %d active scans.\n", count)
	return nil
}

// GetScanProgress returns progress, ETA, and files/sec for a scan job
func (m *Manager) GetScanProgress(jobID uint) (progress float64, eta string, filesPerSec float64, err error) {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()
	
	if !exists {
		return 0, "", 0, fmt.Errorf("no active scanner for job %d", jobID)
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
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no active scanner for job %d", jobID)
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
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.pluginManager = pluginMgr
	
	// Update plugin manager in all existing scanners
	for jobID, scanner := range m.scanners {
		scanner.pluginManager = pluginMgr
		fmt.Printf("INFO: Updated plugin manager for scanner job %d\n", jobID)
	}
	
	fmt.Printf("INFO: Scanner manager plugin manager updated, affected %d active scanners\n", len(m.scanners))
}
