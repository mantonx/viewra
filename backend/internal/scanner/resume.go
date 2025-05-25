package scanner

import (
	"fmt"
	"time"

	"github.com/yourusername/viewra/internal/database"
)

// ResumeScan resumes a previously paused scan
func (m *Manager) ResumeScan(jobID uint) (*database.ScanJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Get the scan job from database
	var scanJob database.ScanJob
	if err := m.db.Preload("Library").First(&scanJob, jobID).Error; err != nil {
		return nil, fmt.Errorf("failed to load scan job: %w", err)
	}
	
	if scanJob.Status != "paused" {
		return nil, fmt.Errorf("scan job is not paused (current status: %s)", scanJob.Status)
	}
	
	// Update job status to running
	now := time.Now()
	scanJob.Status = "running"
	scanJob.ResumedAt = &now
	scanJob.CompletedAt = nil // Clear completed at since it's running again
	
	if err := m.db.Save(&scanJob).Error; err != nil {
		return nil, fmt.Errorf("failed to update scan job status: %w", err)
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
		
		// Resume from where it left off
		if err := scanner.Resume(scanJob.LibraryID); err != nil {
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
