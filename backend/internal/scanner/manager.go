package scanner

import (
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/viewra/internal/database"
	"gorm.io/gorm"
)

// Manager handles multiple concurrent file scanning operations
type Manager struct {
	db       *gorm.DB
	scanners map[uint]*FileScanner // jobID -> scanner
	mu       sync.RWMutex
}

// NewManager creates a new scanner manager
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:       db,
		scanners: make(map[uint]*FileScanner),
	}
}

// StartScan creates and starts a new scan job for a library
func (m *Manager) StartScan(libraryID uint) (*database.ScanJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if library exists
	var library database.MediaLibrary
	if err := m.db.First(&library, libraryID).Error; err != nil {
		return nil, fmt.Errorf("library not found: %w", err)
	}
	
	// Check if there's already a running scan for this library
	var existingJob database.ScanJob
	err := m.db.Where("library_id = ? AND status IN ?", libraryID, []string{"pending", "running"}).First(&existingJob).Error
	if err == nil {
		return nil, fmt.Errorf("scan already running for library %d", libraryID)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Create new scan job
	scanJob := database.ScanJob{
		LibraryID: libraryID,
		Status:    "pending",
	}
	
	if err := m.db.Create(&scanJob).Error; err != nil {
		return nil, fmt.Errorf("failed to create scan job: %w", err)
	}
	
	// Create and start scanner
	scanner := NewFileScanner(m.db, scanJob.ID)
	m.scanners[scanJob.ID] = scanner
	
	go func() {
		defer func() {
			// Only remove scanner from map if scan completed or failed (not paused)
			var currentJob database.ScanJob
			if err := m.db.First(&currentJob, scanJob.ID).Error; err == nil {
				if currentJob.Status == "completed" || currentJob.Status == "failed" {
					m.RemoveCompletedScanner(scanJob.ID)
				}
			}
		}()
		
		if err := scanner.Start(libraryID); err != nil {
			// Check if this was a pause or an actual error
			if err.Error() == "scan paused" {
				// Don't update status here, StopScan already handled it
				return
			}
			
			// Update job with error
			m.mu.Lock()
			if err := m.db.First(&scanJob, scanJob.ID).Error; err == nil {
				scanJob.Status = "failed"
				scanJob.ErrorMessage = err.Error()
				now := time.Now()
				scanJob.CompletedAt = &now
				m.db.Save(&scanJob)
			}
			m.mu.Unlock()
		}
	}()
	
	return &scanJob, nil
}

// StopScan pauses a running scan job
func (m *Manager) StopScan(jobID uint) error {
	m.mu.RLock()
	scanner, exists := m.scanners[jobID]
	m.mu.RUnlock()
	
	if !exists {
		// Check if the job exists in the database but isn't running
		var scanJob database.ScanJob
		if err := m.db.First(&scanJob, jobID).Error; err != nil {
			return fmt.Errorf("scan job %d not found", jobID)
		}
		
		// If the job exists but isn't running, just update its status
		if scanJob.Status == "running" {
			scanJob.Status = "paused"
			return m.db.Save(&scanJob).Error
		}
		
		return fmt.Errorf("scan job %d exists but is not running (current status: %s)", jobID, scanJob.Status)
	}
	
	scanner.Pause()
	
	// Update job status
	var scanJob database.ScanJob
	if err := m.db.First(&scanJob, jobID).Error; err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}
	
	scanJob.Status = "paused"
	// Don't set CompletedAt for paused scans - they're not completed
	
	return m.db.Save(&scanJob).Error
}

// GetScanStatus returns the current status of a scan job
func (m *Manager) GetScanStatus(jobID uint) (*database.ScanJob, error) {
	var scanJob database.ScanJob
	err := m.db.Preload("Library").First(&scanJob, jobID).Error
	if err != nil {
		return nil, fmt.Errorf("scan job not found: %w", err)
	}
	
	return &scanJob, nil
}

// GetAllScans returns all scan jobs
func (m *Manager) GetAllScans() ([]database.ScanJob, error) {
	var scanJobs []database.ScanJob
	err := m.db.Preload("Library").Order("created_at DESC").Find(&scanJobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get scan jobs: %w", err)
	}
	
	return scanJobs, nil
}

// CancelAllScans stops all running scans and marks them as paused
// This is useful for graceful shutdowns or when the system needs to be restarted
func (m *Manager) CancelAllScans() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	count := 0
	for jobID, scanner := range m.scanners {
		// Pause the scanner
		scanner.Pause()
		
		// Update job status
		var scanJob database.ScanJob
		if err := m.db.First(&scanJob, jobID).Error; err != nil {
			// Log error but continue with others
			fmt.Printf("Error updating scan job %d: %v\n", jobID, err)
			continue
		}
		
		scanJob.Status = "paused"
		if err := m.db.Save(&scanJob).Error; err != nil {
			fmt.Printf("Error saving scan job %d: %v\n", jobID, err)
			continue
		}
		
		count++
	}
	
	// Don't delete scanners from the map - just leave them paused
	// so they can be resumed later if needed
	
	return count, nil
}

// CleanupOldJobs removes old completed/failed scan jobs (older than 30 days)
func (m *Manager) CleanupOldJobs() error {
	cutoff := time.Now().AddDate(0, 0, -30) // 30 days ago
	
	result := m.db.Where("status IN ? AND completed_at < ?", []string{"completed", "failed", "paused"}, cutoff).Delete(&database.ScanJob{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup old scan jobs: %w", result.Error)
	}
	
	fmt.Printf("Cleaned up %d old scan jobs\n", result.RowsAffected)
	return nil
}

// GetLibraryStats returns statistics for a library's scanned files
func (m *Manager) GetLibraryStats(libraryID uint) (map[string]interface{}, error) {
	var stats struct {
		TotalFiles int64 `json:"total_files"`
		TotalSize  int64 `json:"total_size"`
	}
	
	err := m.db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("COUNT(*) as total_files, COALESCE(SUM(size), 0) as total_size").
		Scan(&stats).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get library stats: %w", err)
	}
	
	// Get file extension breakdown
	var extensionStats []struct {
		Extension string `json:"extension"`
		Count     int64  `json:"count"`
	}
	
	err = m.db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("LOWER(SUBSTR(path, LENGTH(path) - INSTR(REVERSE(path), '.') + 1)) as extension, COUNT(*) as count").
		Group("extension").
		Order("count DESC").
		Limit(10).
		Scan(&extensionStats).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get extension stats: %w", err)
	}
	
	return map[string]interface{}{
		"total_files":      stats.TotalFiles,
		"total_size":       stats.TotalSize,
		"extension_stats":  extensionStats,
	}, nil
}

// RemoveCompletedScanner removes a scanner from the active scanners map
// This should be called when a scan completes or fails (but not when paused)
func (m *Manager) RemoveCompletedScanner(jobID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scanners, jobID)
}
