package scanner

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// Manager manages scanning operations with robust state management and crash recovery.
//
// ROBUSTNESS FEATURES:
//
// 1. STATE SYNCHRONIZATION:
//    - Maintains consistency between in-memory scanner state and database
//    - Background synchronizer detects and auto-fixes state mismatches
//    - Periodic health checks prevent state drift
//
// 2. CRASH RECOVERY:
//    - Automatically detects "orphaned" jobs from backend restarts
//    - Intelligent auto-resume for jobs with significant progress
//    - Cleanup of duplicate jobs for the same library
//    - Validation that libraries still exist before recovery
//
// 3. API CONSISTENCY:
//    - Library-based pause/resume methods eliminate job ID confusion
//    - Consistent error handling and validation
//    - Graceful fallback behaviors (auto-start if no paused scan)
//
// 4. FAULT TOLERANCE:
//    - Graceful handling of scanner crashes or interruptions
//    - Timeout-based cleanup for stuck operations
//    - Resource cleanup on library deletion
//    - Protection against concurrent operations on same library
//
// 5. ENHANCED SAFEGUARDS:
//    - Transactional operations with rollback capabilities
//    - Distributed locking to prevent race conditions
//    - Comprehensive validation and verification
//    - Automated monitoring and self-healing
//
// This design ensures the scanner remains robust across backend restarts,
// network issues, system crashes, and other failure scenarios.
type Manager struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager plugins.Manager
	safeguards    *SafeguardSystem
	mu            sync.RWMutex
	scanners      map[uint32]*LibraryScanner // jobID -> scanner mapping
	stopChannels  map[uint32]chan struct{}   // jobID -> stop channel mapping
	workers       int
	done          chan struct{}
	cleanupTicker *time.Ticker
	monitorTicker *time.Ticker
	// performance tracking
	workerPool   *utils.WorkerPool
	rateLimiter  *utils.RateLimiter
	lastCleanup  time.Time
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// File monitoring
	fileMonitor   *FileMonitor
}

// NewManager creates a new scanner manager
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager, opts *ManagerOptions) *Manager {
	// Default options
	if opts == nil {
		opts = &ManagerOptions{
			Workers:      runtime.NumCPU(),
			CleanupHours: 24,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Create file monitor
	fileMonitor, err := NewFileMonitor(db, eventBus, pluginManager)
	if err != nil {
		logger.Error("Failed to create file monitor", "error", err)
		// Continue without file monitoring
		fileMonitor = nil
	}
	
	manager := &Manager{
		db:            db,
		eventBus:      eventBus,
		pluginManager: pluginManager,
		scanners:      make(map[uint32]*LibraryScanner),
		stopChannels:  make(map[uint32]chan struct{}),
		workers:       opts.Workers,
		done:          make(chan struct{}),
		workerPool:    utils.NewWorkerPool(opts.Workers),
		rateLimiter:   utils.NewRateLimiter(10, time.Second), // 10 operations per second
		ctx:           ctx,
		cancel:        cancel,
		fileMonitor:   fileMonitor,
	}
	
	// Initialize safeguards system
	manager.safeguards = NewSafeguardSystem(db, eventBus, manager)
	
	return manager
}

// recoverOrphanedJobs handles scan jobs that were marked as "running" when the backend restarted
// and automatically resumes paused jobs that have made progress
func (m *Manager) recoverOrphanedJobs() error {
	logger.Info("Starting orphaned job recovery process...")
	
	// STEP 1: Find all jobs that were "running" but lost their in-memory state
	var orphanedJobs []database.ScanJob
	if err := m.db.Where("status = ?", "running").Preload("Library").Find(&orphanedJobs).Error; err != nil {
		return fmt.Errorf("failed to query orphaned jobs: %w", err)
	}

	// STEP 2: Find paused jobs that could potentially be auto-resumed
	var pausedJobs []database.ScanJob
	if err := m.db.Where("status = ? AND files_processed > 0", "paused").Preload("Library").Find(&pausedJobs).Error; err != nil {
		logger.Warn("Failed to query paused jobs for auto-resume: %v", err)
	}

	// STEP 3: Detect and clean up duplicate jobs for the same library
	if err := m.cleanupDuplicateJobs(); err != nil {
		logger.Error("Failed to cleanup duplicate jobs: %v", err)
		// Continue with recovery even if cleanup fails
	}

	if len(orphanedJobs) == 0 && len(pausedJobs) == 0 {
		logger.Info("No orphaned or resumable scan jobs found")
		return nil
	}

	// STEP 4: Handle orphaned running jobs - mark as paused for potential resume
	if len(orphanedJobs) > 0 {
		logger.Info("Found %d orphaned scan jobs from backend restart, initiating recovery...", len(orphanedJobs))

		for _, job := range orphanedJobs {
			// Validate that the library still exists
			if job.Library.ID == 0 {
				logger.Warn("Orphaned job %d references non-existent library, marking as failed", job.ID)
				if err := m.db.Model(&database.ScanJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
					"status": string(utils.StatusFailed),
					"error_message": "Library no longer exists",
				}).Error; err != nil {
					logger.Error("Failed to mark orphaned job as failed: %v", err)
				}
				continue
			}

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

	// STEP 5: Auto-resume paused jobs based on configurable criteria
	if len(pausedJobs) > 0 {
		logger.Info("Checking %d paused scan jobs for auto-resume eligibility...", len(pausedJobs))

		autoResumedCount := 0
		for _, job := range pausedJobs {
			// Skip if library doesn't exist
			if job.Library.ID == 0 {
				continue
			}

			// Auto-resume jobs that have processed at least 10 files or 1% progress
			shouldAutoResume := job.FilesProcessed >= 10 ||
				(job.FilesFound > 0 && float64(job.FilesProcessed)/float64(job.FilesFound) >= 0.01)

			if shouldAutoResume {
				logger.Info("🔄 Auto-resuming scan job %d (processed %d/%d files, %.1f%% complete)",
					job.ID, job.FilesProcessed, job.FilesFound, job.Progress)

				// Resume the job (this will properly sync in-memory state)
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

// cleanupDuplicateJobs removes duplicate scan jobs for the same library, keeping only the most recent/relevant one
func (m *Manager) cleanupDuplicateJobs() error {
	// Find all libraries that have multiple scan jobs
	type libraryJobCount struct {
		LibraryID uint `json:"library_id"`
		JobCount  int  `json:"job_count"`
	}
	
	var libraryCounts []libraryJobCount
	if err := m.db.Model(&database.ScanJob{}).
		Select("library_id, COUNT(*) as job_count").
		Group("library_id").
		Having("COUNT(*) > 1").
		Scan(&libraryCounts).Error; err != nil {
		return fmt.Errorf("failed to find duplicate jobs: %w", err)
	}

	if len(libraryCounts) == 0 {
		return nil
	}

	logger.Info("Found %d libraries with multiple scan jobs, cleaning up duplicates...", len(libraryCounts))

	for _, libCount := range libraryCounts {
		// Get all jobs for this library, ordered by creation time
		var jobs []database.ScanJob
		if err := m.db.Where("library_id = ?", libCount.LibraryID).
			Order("created_at DESC").
			Find(&jobs).Error; err != nil {
			logger.Error("Failed to get jobs for library %d: %v", libCount.LibraryID, err)
			continue
		}

		if len(jobs) <= 1 {
			continue
		}

		// Determine which job to keep based on priority:
		// 1. Running jobs
		// 2. Paused jobs with progress
		// 3. Most recent job
		var jobToKeep *database.ScanJob
		var jobsToDelete []database.ScanJob

		// First pass: find running jobs
		for i := range jobs {
			if jobs[i].Status == "running" {
				if jobToKeep == nil {
					jobToKeep = &jobs[i]
				} else {
					// Multiple running jobs - keep the most recent
					if jobs[i].CreatedAt.After(jobToKeep.CreatedAt) {
						jobsToDelete = append(jobsToDelete, *jobToKeep)
						jobToKeep = &jobs[i]
					} else {
						jobsToDelete = append(jobsToDelete, jobs[i])
					}
				}
			}
		}

		// If no running jobs, find paused jobs with progress
		if jobToKeep == nil {
			for i := range jobs {
				if jobs[i].Status == "paused" && jobs[i].FilesProcessed > 0 {
					if jobToKeep == nil {
						jobToKeep = &jobs[i]
					} else {
						// Keep the one with more progress
						if jobs[i].FilesProcessed > jobToKeep.FilesProcessed {
							jobsToDelete = append(jobsToDelete, *jobToKeep)
							jobToKeep = &jobs[i]
						} else {
							jobsToDelete = append(jobsToDelete, jobs[i])
						}
					}
				}
			}
		}

		// If still no keeper, keep the most recent job
		if jobToKeep == nil {
			jobToKeep = &jobs[0] // Already ordered by created_at DESC
		}

		// Mark all other jobs for deletion
		for i := range jobs {
			if jobs[i].ID != jobToKeep.ID {
				found := false
				for j := range jobsToDelete {
					if jobsToDelete[j].ID == jobs[i].ID {
						found = true
						break
					}
				}
				if !found {
					jobsToDelete = append(jobsToDelete, jobs[i])
				}
			}
		}

		// Delete duplicate jobs
		for _, job := range jobsToDelete {
			logger.Info("Removing duplicate scan job %d for library %d (keeping job %d)", 
				job.ID, libCount.LibraryID, jobToKeep.ID)
			
			if err := m.db.Delete(&job).Error; err != nil {
				logger.Error("Failed to delete duplicate job %d: %v", job.ID, err)
			}
		}

		logger.Info("Cleaned up %d duplicate jobs for library %d, kept job %d", 
			len(jobsToDelete), libCount.LibraryID, jobToKeep.ID)
	}

	return nil
}

// StartScan creates and starts a new scan job for the specified library.
// It validates that no scan is already running for the library before starting.
func (m *Manager) StartScan(libraryID uint32) (*database.ScanJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Perform cleanup before starting scan
	logger.Info("Performing pre-scan cleanup", "library_id", libraryID)
	if err := m.CleanupLibraryData(libraryID); err != nil {
		logger.Warn("Pre-scan cleanup had issues", "library_id", libraryID, "error", err)
		// Continue with scan even if cleanup had issues
	}

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
func (m *Manager) runScanJob(scanner *LibraryScanner, jobID, libraryID uint32, isResume bool) {
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
					if err := m.fileMonitor.StartMonitoring(uint(libraryID), uint(jobID)); err != nil {
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
func (m *Manager) StopScan(jobID uint32) error {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

	if !exists {
		return m.handleInactiveJobStop(jobID)
	}

	// Get job details for the event
	var scanJob database.ScanJob
	var libraryID uint32
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
func (m *Manager) TerminateScan(jobID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scanner, exists := m.scanners[jobID]
	if !exists {
		// Job is not active, just update database status
		return m.handleInactiveJobTermination(jobID)
	}

	// Get job details for the event
	var scanJob database.ScanJob
	var libraryID uint32
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
func (m *Manager) handleInactiveJobStop(jobID uint32) error {
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
func (m *Manager) handleInactiveJobTermination(jobID uint32) error {
	var scanJob database.ScanJob
	if err := m.db.First(&scanJob, jobID).Error; err != nil {
		return fmt.Errorf("scan job not found: %w", err)
	}

	// Update status to cancelled/failed
	return utils.UpdateJobStatus(m.db, jobID, utils.StatusFailed, "Terminated during library deletion")
}

// ResumeScan resumes a previously paused scan job.
func (m *Manager) ResumeScan(jobID uint32) error {
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
func (m *Manager) GetScanStatus(jobID uint32) (*database.ScanJob, error) {
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
func (m *Manager) CleanupJobsByLibrary(libraryID uint32) (int64, error) {
	// First, identify scanners to stop (without holding the lock)
	var scannersToStop []struct {
		jobID   uint32
		scanner *LibraryScanner
	}

	m.mu.RLock()
	for jobID, scanner := range m.scanners {
		// Get the job to check library ID
		var scanJob database.ScanJob
		if err := m.db.First(&scanJob, jobID).Error; err == nil {
			if scanJob.LibraryID == libraryID {
				scannersToStop = append(scannersToStop, struct {
					jobID   uint32
					scanner *LibraryScanner
				}{jobID, scanner})
			}
		}
	}
	m.mu.RUnlock()

	// Stop scanners without holding the manager lock (to avoid deadlock)
	var stoppedScanners []uint32
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
	var existingLibraryIDs []uint32
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
		jobID   uint32
		scanner *LibraryScanner
	}

	m.mu.RLock()
	for _, job := range orphanedJobs {
		if scanner, exists := m.scanners[job.ID]; exists {
			scannersToStop = append(scannersToStop, struct {
				jobID   uint32
				scanner *LibraryScanner
			}{job.ID, scanner})
		}
	}
	m.mu.RUnlock()

	// Stop scanners without holding the manager lock (to avoid deadlock)
	var stoppedScanners []uint32
	for _, item := range scannersToStop {
		// Find the corresponding job for library ID
		var libraryID uint32
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
	var orphanedJobIDs []uint32
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
func (m *Manager) GetLibraryStats(libraryID uint32) (*utils.LibraryStats, error) {
	return utils.GetLibraryStatistics(m.db, libraryID)
}

// removeScanner safely removes a scanner from the active scanners map.
func (m *Manager) removeScanner(jobID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
func (m *Manager) GetScanProgress(jobID uint32) (progress float64, eta string, filesPerSec float64, err error) {
	// Safety check for nil manager
	if m == nil {
		return 0, "", 0, fmt.Errorf("scanner manager is nil")
	}
	
	// Safety check for nil mutex
	if m.scanners == nil {
		return 0, "", 0, fmt.Errorf("scanner manager scanners map is nil")
	}
	
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

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

// GetDetailedScanProgress returns detailed metrics including adaptive throttling data
func (m *Manager) GetDetailedScanProgress(jobID uint32) (map[string]interface{}, error) {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan job %d not found or not running", jobID)
	}

	// Get detailed metrics from the adaptive throttler
	throttleLimits := scanner.adaptiveThrottler.GetCurrentLimits()
	systemMetrics := scanner.adaptiveThrottler.GetSystemMetrics()
	networkStats := scanner.adaptiveThrottler.GetNetworkStats()
	throttleConfig := scanner.adaptiveThrottler.GetThrottleConfig()

	// Check if emergency brake is active
	shouldThrottle, throttleDelay := scanner.adaptiveThrottler.ShouldThrottle()
	emergencyBrakeActive := shouldThrottle && throttleDelay > 100*time.Millisecond

	// Get container awareness info
	containerAware := scanner.adaptiveThrottler.isContainerized
	var containerLimits ContainerLimits
	if containerAware {
		containerLimits = scanner.adaptiveThrottler.containerLimits
	}

	// Get worker and queue statistics
	activeWorkers, minWorkers, maxWorkers, queueLen := scanner.GetWorkerStats()

	// Get progress and ETA information from the progress estimator
	var progress float64
	var eta string
	var estimatedTimeLeft int
	var filesPerSec float64
	if scanner.progressEstimator != nil {
		progressVal, etaTime, filesPerSecVal := scanner.progressEstimator.GetEstimate()
		progress = progressVal
		filesPerSec = filesPerSecVal
		
		// Validate ETA before using it
		if !etaTime.IsZero() {
			// Check if ETA is reasonable (not more than 1 year in the future)
			now := time.Now()
			maxReasonableETA := now.Add(365 * 24 * time.Hour) // 1 year
			
			if etaTime.After(now) && etaTime.Before(maxReasonableETA) {
				eta = etaTime.Format(time.RFC3339)
				// Calculate seconds remaining from backend time (authoritative)
				estimatedTimeLeft = int(time.Until(etaTime).Seconds())
				if estimatedTimeLeft < 0 {
					estimatedTimeLeft = 0 // Don't show negative time
				}
			}
			// If ETA is unreasonable, leave eta and estimatedTimeLeft as empty/zero
		}
	}

	// Build comprehensive metrics map
	detailedMetrics := map[string]interface{}{
		// Basic scan metrics
		"job_id":           jobID,
		"files_processed":  scanner.filesProcessed.Load(),
		"files_found":      scanner.filesFound.Load(),
		"files_skipped":    scanner.filesSkipped.Load(),
		"bytes_processed":  scanner.bytesProcessed.Load(),
		"bytes_found":      scanner.bytesFound.Load(), // Total bytes discovered during scan
		"errors_count":     scanner.errorsCount.Load(),

		// Progress and ETA information
		"progress":         progress,
		"eta":              eta,
		"estimated_time_left": estimatedTimeLeft,
		"files_per_second": filesPerSec,

		// Additional total information for better UI display
		"total_files":      scanner.filesFound.Load(), // Alias for files_found
		"total_bytes":      scanner.progressEstimator.GetTotalBytes(), // CRITICAL: Total expected bytes for UI
		"remaining_files":  scanner.filesFound.Load() - scanner.filesProcessed.Load(),

		// Worker statistics
		"active_workers":   activeWorkers,
		"min_workers":      minWorkers,
		"max_workers":      maxWorkers,
		"queue_length":     queueLen,

		// System metrics from adaptive throttler
		"cpu_percent":      systemMetrics.CPUPercent,
		"memory_percent":   systemMetrics.MemoryPercent,
		"memory_used_mb":   systemMetrics.MemoryUsedMB,
		"io_wait_percent":  systemMetrics.IOWaitPercent,
		"load_average":     systemMetrics.LoadAverage,
		"network_mbps":     systemMetrics.NetworkUtilMBps,
		"disk_read_mbps":   systemMetrics.DiskReadMBps,
		"disk_write_mbps":  systemMetrics.DiskWriteMBps,
		"metrics_timestamp": systemMetrics.TimestampUTC,

		// Throttling configuration and limits
		"throttle_enabled":           throttleLimits.Enabled,
		"current_batch_size":         throttleLimits.BatchSize,
		"processing_delay_ms":        throttleLimits.ProcessingDelay.Milliseconds(),
		"network_bandwidth_limit":    throttleLimits.NetworkBandwidth,
		"io_throttle_percent":        throttleLimits.IOThrottle,
		"emergency_brake":            emergencyBrakeActive,

		// Network health metrics (important for NFS performance)
		"dns_latency_ms":      networkStats.DNSLatencyMs,
		"network_latency_ms":  networkStats.NetworkLatencyMs,
		"packet_loss_percent": networkStats.PacketLossPercent,
		"connection_errors":   networkStats.ConnectionErrors,
		"network_healthy":     networkStats.IsHealthy,
		"last_health_check":   networkStats.LastHealthCheck,

		// Throttling configuration
		"target_cpu_percent":        throttleConfig.TargetCPUPercent,
		"max_cpu_percent":           throttleConfig.MaxCPUPercent,
		"target_memory_percent":     throttleConfig.TargetMemoryPercent,
		"max_memory_percent":        throttleConfig.MaxMemoryPercent,
		"target_network_mbps":       throttleConfig.TargetNetworkThroughput,
		"max_network_mbps":          throttleConfig.MaxNetworkThroughput,
		"emergency_brake_threshold": throttleConfig.EmergencyBrakeThreshold,

		// Container awareness
		"container_aware": containerAware,
	}

	// Add container-specific metrics if running in a container
	if containerAware {
		detailedMetrics["cgroup_version"] = scanner.adaptiveThrottler.cgroupVersion
		detailedMetrics["container_memory_limited"] = containerLimits.MemoryLimitBytes > 0
		detailedMetrics["container_cpu_limited"] = containerLimits.MaxCPUPercent > 0
		detailedMetrics["container_io_throttled"] = containerLimits.BlkioThrottleRead > 0 || containerLimits.BlkioThrottleWrite > 0

		if containerLimits.MemoryLimitBytes > 0 {
			detailedMetrics["container_memory_limit_gb"] = float64(containerLimits.MemoryLimitBytes) / (1024 * 1024 * 1024)
		}
		if containerLimits.MaxCPUPercent > 0 {
			detailedMetrics["container_cpu_limit_percent"] = containerLimits.MaxCPUPercent
		}
		if containerLimits.BlkioThrottleRead > 0 {
			detailedMetrics["container_blkio_read_bps"] = containerLimits.BlkioThrottleRead
		}
		if containerLimits.BlkioThrottleWrite > 0 {
			detailedMetrics["container_blkio_write_bps"] = containerLimits.BlkioThrottleWrite
		}
	}

	return detailedMetrics, nil
}

// SetPluginManager updates the plugin manager for this scanner manager
// and all currently running scanners
func (m *Manager) SetPluginManager(pm plugins.Manager) {
	m.pluginManager = pm
	
	// Update all active scanners
	m.mu.RLock()
	for _, scanner := range m.scanners {
		scanner.pluginManager = pm
		if scanner.pluginRouter != nil {
			scanner.pluginRouter = NewPluginRouter(pm)
		}
	}
	m.mu.RUnlock()
}

// DisableThrottlingForJob disables adaptive throttling for a specific scan job
func (m *Manager) DisableThrottlingForJob(jobID uint32) error {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scan job %d not found or not running", jobID)
	}

	if scanner.adaptiveThrottler != nil {
		scanner.adaptiveThrottler.DisableThrottling()
		logger.Info("Throttling disabled for scan job", "job_id", jobID)
		return nil
	}

	return fmt.Errorf("no adaptive throttler available for job %d", jobID)
}

// EnableThrottlingForJob re-enables adaptive throttling for a specific scan job
func (m *Manager) EnableThrottlingForJob(jobID uint32) error {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scan job %d not found or not running", jobID)
	}

	if scanner.adaptiveThrottler != nil {
		scanner.adaptiveThrottler.EnableThrottling()
		logger.Info("Throttling re-enabled for scan job", "job_id", jobID)
		return nil
	}

	return fmt.Errorf("no adaptive throttler available for job %d", jobID)
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
func (m *Manager) GetMonitoringStatus() map[uint32]*MonitoredLibrary {
	if m.fileMonitor == nil {
		return make(map[uint32]*MonitoredLibrary)
	}
	
	// Convert from map[uint] to map[uint32]
	sourceMap := m.fileMonitor.GetMonitoringStatus()
	resultMap := make(map[uint32]*MonitoredLibrary)
	for libraryID, monitoredLib := range sourceMap {
		resultMap[uint32(libraryID)] = monitoredLib
	}
	return resultMap
}

// CleanupOrphanedAssets removes assets that reference non-existent media files
// Note: This is now deprecated as asset cleanup is handled by the entity-based asset system
func (m *Manager) CleanupOrphanedAssets() (int, int, error) {
	logger.Info("Orphaned asset cleanup is now handled by the entity-based asset system")
	logger.Info("Use the /api/v1/assets/cleanup endpoint for asset cleanup")
	return 0, 0, nil
}

// CleanupOrphanedFiles removes asset files from disk that have no corresponding database records
// Note: This is now deprecated as asset cleanup is handled by the entity-based asset system
func (m *Manager) CleanupOrphanedFiles() (int, error) {
	logger.Info("Orphaned file cleanup is now handled by the entity-based asset system")
	logger.Info("Use the /api/v1/assets/cleanup endpoint for asset cleanup")
	return 0, nil
}

// CleanupScanJob removes a scan job and all its discovered files and assets
func (m *Manager) CleanupScanJob(scanJobID uint32) error {
	if m.safeguards == nil {
		return fmt.Errorf("safeguard system not initialized")
	}
	
	// Use safeguarded deletion which handles comprehensive cleanup
	result, err := m.safeguards.DeleteSafeguardedScan(scanJobID)
	if err != nil {
		return fmt.Errorf("failed to cleanup scan job: %w", err)
	}
	
	if !result.Success {
		return fmt.Errorf("scan job cleanup failed: %s", result.Message)
	}
	
	logger.Info("Scan job completely removed", "scan_job_id", scanJobID)
	return nil
}

// PauseScanByLibrary pauses any running scan for the specified library
// This method provides a consistent API that always uses library IDs
func (m *Manager) PauseScanByLibrary(libraryID uint32) error {
	// Find the currently running job for this library
	var runningJob database.ScanJob
	if err := m.db.Where("library_id = ? AND status = ?", libraryID, "running").First(&runningJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("no running scan found for library %d", libraryID)
		}
		return fmt.Errorf("failed to find running scan for library %d: %w", libraryID, err)
	}

	// Use the existing StopScan method with the job ID
	return m.StopScan(runningJob.ID)
}

// ResumeScanByLibrary resumes the most recent paused scan for the specified library
// This method provides a consistent API that always uses library IDs
func (m *Manager) ResumeScanByLibrary(libraryID uint32) error {
	// Find the most recent paused job for this library
	var pausedJob database.ScanJob
	if err := m.db.Where("library_id = ? AND status = ?", libraryID, "paused").
		Order("updated_at DESC").
		First(&pausedJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// No paused scan found, try to start a new one
			_, err := m.StartScan(libraryID)
			return err
		}
		return fmt.Errorf("failed to find paused scan for library %d: %w", libraryID, err)
	}

	// Use the existing ResumeScan method with the job ID
	return m.ResumeScan(pausedJob.ID)
}

// GetLibraryScanStatus returns the status of the most relevant scan job for a library
// This returns the currently running job, or the most recent paused/completed job
func (m *Manager) GetLibraryScanStatus(libraryID uint32) (*database.ScanJob, error) {
	// First try to find a running job
	var scanJob database.ScanJob
	err := m.db.Where("library_id = ? AND status = ?", libraryID, "running").
		Preload("Library").
		First(&scanJob).Error
	
	if err == nil {
		return &scanJob, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to query running jobs for library %d: %w", libraryID, err)
	}

	// No running job, find the most recent job of any status
	err = m.db.Where("library_id = ?", libraryID).
		Preload("Library").
		Order("updated_at DESC").
		First(&scanJob).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no scan jobs found for library %d", libraryID)
		}
		return nil, fmt.Errorf("failed to find scan jobs for library %d: %w", libraryID, err)
	}

	return &scanJob, nil
}

// GetActiveLibraries returns a list of library IDs that have active (running) scans
func (m *Manager) GetActiveLibraries() ([]uint32, error) {
	var libraryIDs []uint32
	if err := m.db.Model(&database.ScanJob{}).
		Where("status = ?", "running").
		Distinct("library_id").
		Pluck("library_id", &libraryIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get active libraries: %w", err)
	}
	return libraryIDs, nil
}

// StartStateSynchronizer runs a background goroutine that periodically checks
// for inconsistencies between database and in-memory scanner state
func (m *Manager) StartStateSynchronizer() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				logger.Debug("State synchronizer stopping due to context cancellation")
				return
			case <-ticker.C:
				if err := m.synchronizeState(); err != nil {
					logger.Error("State synchronization failed: %v", err)
				}
			}
		}
	}()
	
	logger.Info("Started background state synchronizer")
}

// synchronizeState checks for inconsistencies between database and in-memory state
func (m *Manager) synchronizeState() error {
	m.mu.RLock()
	inMemoryJobs := make(map[uint32]bool)
	for jobID := range m.scanners {
		inMemoryJobs[jobID] = true
	}
	m.mu.RUnlock()

	// Find database jobs marked as "running"
	var runningJobs []database.ScanJob
	if err := m.db.Where("status = ?", "running").Find(&runningJobs).Error; err != nil {
		return fmt.Errorf("failed to query running jobs: %w", err)
	}

	// Check for inconsistencies
	var inconsistencies []string
	for _, job := range runningJobs {
		if !inMemoryJobs[job.ID] {
			inconsistencies = append(inconsistencies, fmt.Sprintf("Job %d marked as running in DB but not in memory", job.ID))
			
			// Auto-fix: mark as paused
			if err := utils.UpdateJobStatus(m.db, job.ID, utils.StatusPaused, "State sync: scanner not found in memory"); err != nil {
				logger.Error("Failed to auto-fix inconsistent job %d: %v", job.ID, err)
			} else {
				logger.Info("Auto-fixed inconsistent job %d: marked as paused", job.ID)
			}
		}
	}

	// Check for in-memory scanners without valid database jobs
	for jobID := range inMemoryJobs {
		var job database.ScanJob
		if err := m.db.First(&job, jobID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				inconsistencies = append(inconsistencies, fmt.Sprintf("In-memory scanner for job %d but no database record", jobID))
				
				// Auto-fix: remove from memory
				m.mu.Lock()
				delete(m.scanners, jobID)
				m.mu.Unlock()
				logger.Info("Auto-fixed orphaned in-memory scanner for job %d", jobID)
			}
		} else if job.Status != "running" {
			inconsistencies = append(inconsistencies, fmt.Sprintf("In-memory scanner for job %d but DB status is %s", jobID, job.Status))
		}
	}

	if len(inconsistencies) > 0 {
		logger.Warn("Found %d state inconsistencies, auto-fixing...", len(inconsistencies))
		for _, issue := range inconsistencies {
			logger.Debug("State inconsistency: %s", issue)
		}
	}

	return nil
}

// StartSafeguardedScan provides a safeguarded way to start scans with enhanced reliability
func (m *Manager) StartSafeguardedScan(libraryID uint32) (*OperationResult, error) {
	if m.safeguards == nil {
		return nil, fmt.Errorf("safeguard system not initialized")
	}
	return m.safeguards.StartSafeguardedScan(libraryID)
}

// PauseSafeguardedScan provides a safeguarded way to pause scans with enhanced reliability
func (m *Manager) PauseSafeguardedScan(jobID uint32) (*OperationResult, error) {
	if m.safeguards == nil {
		return nil, fmt.Errorf("safeguard system not initialized")
	}
	return m.safeguards.PauseSafeguardedScan(jobID)
}

// DeleteSafeguardedScan provides a safeguarded way to delete scans with enhanced reliability
func (m *Manager) DeleteSafeguardedScan(jobID uint32) (*OperationResult, error) {
	if m.safeguards == nil {
		return nil, fmt.Errorf("safeguard system not initialized")
	}
	return m.safeguards.DeleteSafeguardedScan(jobID)
}

// ResumeSafeguardedScan provides a safeguarded way to resume scans
func (m *Manager) ResumeSafeguardedScan(jobID uint32) (*OperationResult, error) {
	if m.safeguards == nil {
		return nil, fmt.Errorf("safeguard system not initialized")
	}
	
	// For resume, we use the existing ResumeScan method but with enhanced validation
	startTime := time.Now()
	result := &OperationResult{
		Operation: OpResume,
		JobID:     jobID,
	}

	// Validate job exists and can be resumed
	scanJob, err := m.GetScanStatus(jobID)
	if err != nil {
		result.Error = fmt.Errorf("job validation failed: %w", err)
		return result, result.Error
	}

	if scanJob.Status != "paused" {
		result.Error = fmt.Errorf("job %d is not paused (status: %s)", jobID, scanJob.Status)
		return result, result.Error
	}

	// Acquire library lock to prevent conflicts
	if err := m.safeguards.lockManager.AcquireLibraryLock(scanJob.LibraryID); err != nil {
		result.Error = fmt.Errorf("failed to acquire library lock: %w", err)
		return result, result.Error
	}
	defer m.safeguards.lockManager.ReleaseLibraryLock(scanJob.LibraryID)

	// Resume the scan
	if err := m.ResumeScan(jobID); err != nil {
		result.Error = fmt.Errorf("failed to resume scan: %w", err)
		return result, result.Error
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	result.Message = fmt.Sprintf("Scan %d resumed successfully", jobID)

	// Publish safeguard event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			events.EventInfo,
			"Safeguard: scan_resumed_safely",
			fmt.Sprintf("Safeguarded operation completed: scan_resumed_safely"),
		)
		event.Data = map[string]interface{}{
			"job_id":     jobID,
			"library_id": scanJob.LibraryID,
			"duration":   result.Duration.Milliseconds(),
		}
		m.eventBus.PublishAsync(event)
	}

	return result, nil
}

// StartSafeguards initializes and starts the safeguard system
func (m *Manager) StartSafeguards() error {
	if m.safeguards == nil {
		return fmt.Errorf("safeguard system not initialized")
	}
	return m.safeguards.Start()
}

// StopSafeguards gracefully stops the safeguard system
func (m *Manager) StopSafeguards() error {
	if m.safeguards == nil {
		return nil // Nothing to stop
	}
	return m.safeguards.Stop()
}

// GetSafeguardStatus returns information about the safeguard system status
func (m *Manager) GetSafeguardStatus() map[string]interface{} {
	if m.safeguards == nil {
		return map[string]interface{}{
			"enabled": false,
			"error":   "safeguard system not initialized",
		}
	}

	return map[string]interface{}{
		"enabled":                  true,
		"health_check_interval":    m.safeguards.config.HealthCheckInterval.String(),
		"state_validation_interval": m.safeguards.config.StateValidationInterval.String(),
		"cleanup_interval":         m.safeguards.config.CleanupInterval.String(),
		"operation_timeout":        m.safeguards.config.OperationTimeout.String(),
		"orphaned_job_threshold":   m.safeguards.config.OrphanedJobThreshold.String(),
		"emergency_cleanup_enabled": m.safeguards.config.EmergencyCleanupEnabled,
	}
}

// CleanupLibraryData performs comprehensive cleanup of trickplay files and duplicate scan jobs
func (m *Manager) CleanupLibraryData(libraryID uint32) error {
	logger.Info("Starting comprehensive library cleanup", "library_id", libraryID)
	
	// Clean up duplicate scan jobs first
	if err := utils.CleanupDuplicateScanJobs(m.db, libraryID); err != nil {
		logger.Error("Failed to cleanup duplicate scan jobs", "library_id", libraryID, "error", err)
		// Don't fail the whole cleanup for this
	}
	
	// Clean up skipped files (trickplay, subtitles, etc.)
	if err := utils.CleanupSkippedFiles(m.db, libraryID); err != nil {
		logger.Error("Failed to cleanup skipped files", "library_id", libraryID, "error", err)
		// Don't fail the whole cleanup for this
	}
	
	logger.Info("Completed library cleanup", "library_id", libraryID)
	return nil
}
