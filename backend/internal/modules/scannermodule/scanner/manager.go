package scanner

import (
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// Manager handles multiple concurrent file scanning operations.
// It manages the lifecycle of scan jobs and provides a centralized
// interface for starting, stopping, and monitoring scan operations.
type Manager struct {
	db           *gorm.DB
	scanners     map[uint]*ParallelFileScanner // jobID -> scanner mapping
	mu           sync.RWMutex                  // protects scanners map
	eventBus     events.EventBus               // system event bus for notifications
	parallelMode bool                          // whether parallel scanning is enabled
}

// NewManager creates a new scanner manager instance.
func NewManager(db *gorm.DB, eventBus events.EventBus) *Manager {
	return &Manager{
		db:       db,
		scanners: make(map[uint]*ParallelFileScanner),
		eventBus: eventBus,
	}
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
	scanner := NewParallelFileScanner(m.db, scanJob.ID, m.eventBus)
	m.scanners[scanJob.ID] = scanner
	
	// Start scanning in background
	go m.runScanJob(scanner, scanJob.ID, libraryID)

	return scanJob, nil
}

// runScanJob executes a scan job in a goroutine and handles cleanup.
func (m *Manager) runScanJob(scanner *ParallelFileScanner, jobID, libraryID uint) {
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
				completeEvent.Data = map[string]interface{}{
					"libraryId":      libraryID,
					"scanJobId":      jobID,
					"filesProcessed": currentJob.FilesProcessed,
					"bytesProcessed": currentJob.BytesProcessed,
					"duration":       currentJob.CompletedAt.Sub(*currentJob.StartedAt).String(),
				}
				m.eventBus.PublishAsync(completeEvent)
			}
		}
	}()

	// Start the actual scanning process
	if err := scanner.Start(libraryID); err != nil {
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

	// Only allow resuming paused jobs
	if scanJob.Status != string(utils.StatusPaused) {
		return fmt.Errorf("cannot resume scan job with status: %s", scanJob.Status)
	}
	
	// Create and register new scanner
	scanner := NewParallelFileScanner(m.db, jobID, m.eventBus)
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
	go m.runScanJob(scanner, jobID, scanJob.LibraryID)

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
