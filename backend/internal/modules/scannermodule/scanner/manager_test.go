package scanner

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEventBus, setupTestDB, createTestLibrary, and createTestDirectory are now in library_scanner_test.go

func TestNewManager(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}

	manager := NewManager(db, eventBus, nil)

	assert.NotNil(t, manager)
	assert.Equal(t, db, manager.db)
	assert.Equal(t, eventBus, manager.eventBus)
	assert.NotNil(t, manager.scanners)
	assert.Equal(t, 0, len(manager.scanners))
}

func TestStartScan_Success(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start scan
	scanJob, err := manager.StartScan(library.ID)

	assert.NoError(t, err)
	assert.NotNil(t, scanJob)
	assert.Equal(t, library.ID, scanJob.LibraryID)

	// Wait a moment for the scan to actually start
	time.Sleep(100 * time.Millisecond)

	// Check the status from the database (may be pending or running)
	var updatedJob database.ScanJob
	err = db.First(&updatedJob, scanJob.ID).Error
	assert.NoError(t, err)
	// Status should be either pending (just created) or running (already started)
	assert.Contains(t, []string{"pending", "running"}, updatedJob.Status)

	// Verify scanner was created
	manager.scannersMu.RLock()
	scanner, exists := manager.scanners[scanJob.ID]
	manager.scannersMu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, scanner)

	// Verify events were published (Manager + LibraryScanner)
	publishedEvents := eventBus.GetEventsForTest()
	assert.Len(t, publishedEvents, 2, "Expected two scan.started events (one from Manager, one from LibraryScanner)")
	var managerEventFound, scannerEventFound bool
	for _, ev := range publishedEvents {
		if ev.Type == events.EventScanStarted {
			if ev.Source == "system" { // Assuming manager uses "system" as source
				managerEventFound = true
			}
			if ev.Source == "scanner" { // LibraryScanner uses "scanner" as source
				scannerEventFound = true
			}
		}
	}
	assert.True(t, managerEventFound, "Manager's scan.started event not found")
	assert.True(t, scannerEventFound, "LibraryScanner's scan.started event not found")

	// Clean up
	manager.StopScan(scanJob.ID)
}

func TestStartScan_LibraryNotFound(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}

	manager := NewManager(db, eventBus, nil)

	// Try to start scan for non-existent library
	scanJob, err := manager.StartScan(999)

	assert.Error(t, err)
	assert.Nil(t, scanJob)
	assert.Contains(t, err.Error(), "library not found")
}

func TestStartScan_DuplicateScan(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start first scan
	scanJob1, err := manager.StartScan(library.ID)
	assert.NoError(t, err)
	assert.NotNil(t, scanJob1)

	// Try to start second scan for same library
	scanJob2, err := manager.StartScan(library.ID)
	assert.Error(t, err)
	assert.Nil(t, scanJob2)
	assert.Contains(t, err.Error(), "scan already running")

	// Clean up
	manager.StopScan(scanJob1.ID)
}

func TestStopScan_Success(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	// === DB Health Check ===
	var jobCount int64
	dbErr := db.Model(&database.ScanJob{}).Count(&jobCount).Error
	require.NoError(t, dbErr, "DB Health Check: Failed to count ScanJobs, tables might not exist.")
	// === End DB Health Check ===

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start scan
	scanJob, err := manager.StartScan(library.ID)
	require.NoError(t, err)

	// Check scanner is running
	manager.scannersMu.RLock()
	_, exists := manager.scanners[scanJob.ID]
	manager.scannersMu.RUnlock()
	assert.True(t, exists)

	// Stop the scan
	err = manager.StopScan(scanJob.ID)
	assert.NoError(t, err)

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify scan was stopped in database
	var updatedJob database.ScanJob
	err = db.First(&updatedJob, scanJob.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "paused", updatedJob.Status)

	// Check scanner was removed from manager
	manager.scannersMu.RLock()
	_, exists = manager.scanners[scanJob.ID]
	manager.scannersMu.RUnlock()
	assert.False(t, exists)
}

func TestStopScan_JobNotFound(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}

	manager := NewManager(db, eventBus, nil)

	// Try to stop non-existent scan
	err := manager.StopScan(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan job not found")
}

func TestResumeScan_Success(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Create a paused scan job
	pausedJob := &database.ScanJob{
		LibraryID:      library.ID,
		Status:         "paused",
		FilesProcessed: 5,
		Progress:       10,
	}
	err := db.Create(pausedJob).Error
	require.NoError(t, err)

	// Resume the scan
	err = manager.ResumeScan(pausedJob.ID)
	require.NoError(t, err)

	// Check scanner is running
	manager.scannersMu.RLock()
	_, exists := manager.scanners[pausedJob.ID]
	manager.scannersMu.RUnlock()
	assert.True(t, exists)

	// Give some time for resume processing
	time.Sleep(100 * time.Millisecond)

	// Verify scan status is running in database
	var updatedJob database.ScanJob
	err = db.First(&updatedJob, pausedJob.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "running", updatedJob.Status)

	// Clean up
	manager.StopScan(pausedJob.ID)
}

func TestResumeScan_JobNotPaused(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start scan (running state)
	scanJob, err := manager.StartScan(library.ID)
	require.NoError(t, err)

	// Try to resume running scan
	err = manager.ResumeScan(scanJob.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in paused state")

	// Clean up
	manager.StopScan(scanJob.ID)
}

func TestGetScanStatus_Success(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start scan
	scanJob, err := manager.StartScan(library.ID)
	require.NoError(t, err)

	// Get status
	status, err := manager.GetScanStatus(scanJob.ID)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, scanJob.ID, status.ID)
	assert.Equal(t, "running", status.Status)

	// Clean up
	manager.StopScan(scanJob.ID)
}

func TestGetAllScans(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start multiple scans
	scanJob1, err := manager.StartScan(library.ID)
	require.NoError(t, err)

	// Stop first scan and create another library for second scan
	manager.StopScan(scanJob1.ID)

	library2 := createTestLibrary(t, db, testDir)
	scanJob2, err := manager.StartScan(library2.ID)
	require.NoError(t, err)

	// Get all scans
	scans, err := manager.GetAllScans()
	assert.NoError(t, err)
	assert.Len(t, scans, 2)

	// Clean up
	manager.StopScan(scanJob2.ID)
}

func TestGetActiveScanCount(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library1 := createTestLibrary(t, db, testDir)
	library2 := createTestLibrary(t, db, testDir+"2")

	// Initially no active scans
	count := manager.GetActiveScanCount()
	assert.Equal(t, 0, count)

	// Start first scan
	scanJob1, err := manager.StartScan(library1.ID)
	require.NoError(t, err)

	count = manager.GetActiveScanCount()
	assert.Equal(t, 1, count)

	// Start second scan
	scanJob2, err := manager.StartScan(library2.ID)
	require.NoError(t, err)

	count = manager.GetActiveScanCount()
	assert.Equal(t, 2, count)

	// Stop one scan
	manager.StopScan(scanJob1.ID)
	count = manager.GetActiveScanCount()
	assert.Equal(t, 1, count)

	// Clean up
	manager.StopScan(scanJob2.ID)
}

func TestCancelAllScans(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library1 := createTestLibrary(t, db, testDir)
	library2 := createTestLibrary(t, db, testDir+"2")

	// Start multiple scans
	_, err := manager.StartScan(library1.ID)
	require.NoError(t, err)

	_, err = manager.StartScan(library2.ID)
	require.NoError(t, err)

	// Cancel all scans
	canceledCount, err := manager.CancelAllScans()
	assert.NoError(t, err)
	assert.Equal(t, 2, canceledCount)

	// Verify no active scans
	count := manager.GetActiveScanCount()
	assert.Equal(t, 0, count)
}

/*
// Commented out TestParallelMode since these methods don't exist in the current manager
func TestParallelMode(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}

	manager := NewManager(db, eventBus, nil)

	// Test default parallel mode
	assert.False(t, manager.GetParallelMode())

	// Enable parallel mode
	manager.SetParallelMode(true)
	assert.True(t, manager.GetParallelMode())

	// Disable parallel mode
	manager.SetParallelMode(false)
	assert.False(t, manager.GetParallelMode())
}
*/

func TestShutdown(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)
	library := createTestLibrary(t, db, testDir)

	// Start scan
	_, err := manager.StartScan(library.ID)
	require.NoError(t, err)

	// Shutdown manager
	err = manager.Shutdown()
	assert.NoError(t, err)

	// Verify all scanners were stopped
	count := manager.GetActiveScanCount()
	assert.Equal(t, 0, count)
}

func TestRecoverOrphanedJobs(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	library := createTestLibrary(t, db, testDir)

	// Create orphaned job (marked as running)
	orphanedJob := &database.ScanJob{
		LibraryID:      library.ID,
		Status:         "running",
		FilesProcessed: 0,
		Progress:       0,
	}
	err := db.Create(orphanedJob).Error
	require.NoError(t, err)

	// Create paused job with progress
	pausedJob := &database.ScanJob{
		LibraryID:      library.ID,
		Status:         "paused",
		FilesProcessed: 15,
		FilesFound:     100,
		Progress:       15,
	}
	err = db.Create(pausedJob).Error
	require.NoError(t, err)

	// Create manager (should trigger recovery)
	manager := NewManager(db, eventBus, nil)

	// Verify orphaned job was marked as paused
	var updatedOrphanedJob database.ScanJob
	err = db.First(&updatedOrphanedJob, orphanedJob.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "paused", updatedOrphanedJob.Status)

	// Verify paused job with progress was auto-resumed
	// (should have an active scanner)
	time.Sleep(100 * time.Millisecond) // Give time for auto-resume
	count := manager.GetActiveScanCount()
	assert.Equal(t, 1, count)

	// Clean up
	manager.Shutdown()
}

func TestConcurrentOperations(t *testing.T) {
	db := setupTestDB(t)
	eventBus := &MockEventBus{}
	testDir := createTestDirectory(t)

	manager := NewManager(db, eventBus, nil)

	// Create multiple libraries
	var libraries []*database.MediaLibrary
	for i := 0; i < 5; i++ {
		lib := createTestLibrary(t, db, fmt.Sprintf("%s_%d", testDir, i))
		libraries = append(libraries, lib)
	}

	// Start scans concurrently
	var wg sync.WaitGroup
	var scanJobs []*database.ScanJob
	var mu sync.Mutex

	for _, lib := range libraries {
		wg.Add(1)
		go func(library *database.MediaLibrary) {
			defer wg.Done()

			scanJob, err := manager.StartScan(library.ID)
			if err != nil {
				t.Errorf("Failed to start scan: %v", err)
				return
			}

			mu.Lock()
			scanJobs = append(scanJobs, scanJob)
			mu.Unlock()
		}(lib)
	}

	wg.Wait()

	// Verify all scans started
	assert.Len(t, scanJobs, 5)
	assert.Equal(t, 5, manager.GetActiveScanCount())

	// Stop all scans concurrently
	for _, job := range scanJobs {
		wg.Add(1)
		go func(scanJob *database.ScanJob) {
			defer wg.Done()
			err := manager.StopScan(scanJob.ID)
			if err != nil {
				t.Errorf("Failed to stop scan: %v", err)
			}
		}(job)
	}

	wg.Wait()

	// Verify all scans stopped
	assert.Equal(t, 0, manager.GetActiveScanCount())
}
