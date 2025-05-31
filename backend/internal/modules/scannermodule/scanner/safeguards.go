package scanner

import (
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

type LockManager struct {
	libraryLocks map[uint32]*sync.Mutex
	jobLocks     map[uint32]*sync.Mutex
	mu           sync.RWMutex
}

// ManagerOptions configures the scanner manager
type ManagerOptions struct {
	Workers      int
	CleanupHours int
}

// SafeguardSystem provides enhanced reliability and safety for scan operations
type SafeguardSystem struct {
	db          *gorm.DB
	eventBus    events.EventBus
	manager     *Manager
	lockManager *LockManager
	config      *SafeguardConfig
	running     bool
	mu          sync.RWMutex
}

// SafeguardConfig contains configuration for the safeguard system
type SafeguardConfig struct {
	HealthCheckInterval      time.Duration
	StateValidationInterval  time.Duration
	CleanupInterval         time.Duration
	OperationTimeout        time.Duration
	ShutdownTimeout         time.Duration
	MaxRetries              int
	RetryInterval           time.Duration
	OrphanedJobThreshold    time.Duration
	OldCompletedJobRetention time.Duration
	EmergencyCleanupEnabled  bool
	ForceKillTimeout        time.Duration
}

// Operation types for safeguarded operations
type Operation string

const (
	OpStart  Operation = "start"
	OpPause  Operation = "pause"
	OpResume Operation = "resume"
	OpDelete Operation = "delete"
)

type OperationResult struct {
	Success       bool
	Message       string
	Details       map[string]interface{}
	JobID         uint32
	Status        string
	ErrorCode     string
	WarningCount  int
	Timestamp     time.Time
	Operation     Operation
	Duration      time.Duration
	Error         error
	WasRolledBack bool
}

// NewSafeguardSystem creates a new safeguard system
func NewSafeguardSystem(db *gorm.DB, eventBus events.EventBus, manager *Manager) *SafeguardSystem {
	return &SafeguardSystem{
		db:       db,
		eventBus: eventBus,
		manager:  manager,
		lockManager: &LockManager{
			libraryLocks: make(map[uint32]*sync.Mutex),
			jobLocks:     make(map[uint32]*sync.Mutex),
		},
		config: &SafeguardConfig{
			HealthCheckInterval:      30 * time.Second,
			StateValidationInterval:  60 * time.Second,
			CleanupInterval:         5 * time.Minute,
			OperationTimeout:        30 * time.Second,
			ShutdownTimeout:         60 * time.Second,
			MaxRetries:              3,
			RetryInterval:           5 * time.Second,
			OrphanedJobThreshold:    5 * time.Minute,
			OldCompletedJobRetention: 30 * 24 * time.Hour,
			EmergencyCleanupEnabled:  true,
			ForceKillTimeout:        10 * time.Second,
		},
	}
}

func (s *SafeguardSystem) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	return nil
}

func (s *SafeguardSystem) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	return nil
}

func (s *SafeguardSystem) StartSafeguardedScan(libraryID uint32) (*OperationResult, error) {
	result := &OperationResult{
		Operation: OpStart,
		JobID:     0, // Will be set when job is created
		Success:   false,
		Timestamp: time.Now(),
	}
	
	// For now, delegate to the regular scan method
	if s.manager != nil {
		scanJob, err := s.manager.StartScan(libraryID)
		if err != nil {
			result.Error = err
			result.Message = "Failed to start scan"
			return result, err
		}
		
		result.Success = true
		result.JobID = scanJob.ID
		result.Message = "Scan started successfully"
	}
	
	return result, nil
}

func (s *SafeguardSystem) PauseSafeguardedScan(jobID uint32) (*OperationResult, error) {
	result := &OperationResult{
		Operation: OpPause,
		JobID:     jobID,
		Success:   false,
		Timestamp: time.Now(),
	}
	
	// For now, delegate to the regular pause method
	if s.manager != nil {
		err := s.manager.StopScan(jobID)
		if err != nil {
			result.Error = err
			result.Message = "Failed to pause scan"
			return result, err
		}
		
		result.Success = true
		result.Message = "Scan paused successfully"
	}
	
	return result, nil
}

func (s *SafeguardSystem) DeleteSafeguardedScan(jobID uint32) (*OperationResult, error) {
	result := &OperationResult{
		Operation: OpDelete,
		JobID:     jobID,
		Success:   false,
		Timestamp: time.Now(),
	}
	
	// For now, delegate to the regular termination method
	if s.manager != nil {
		err := s.manager.TerminateScan(jobID)
		if err != nil {
			result.Error = err
			result.Message = "Failed to delete scan"
			return result, err
		}
		
		result.Success = true
		result.Message = "Scan deleted successfully"
	}
	
	return result, nil
}

func (s *SafeguardSystem) validateScanStart(libraryID uint32) error {
	// Add validation logic here
	return nil
}

func (s *SafeguardSystem) cleanupExistingJobsForLibrary(tx *gorm.DB, libraryID uint32) error {
	// Clean up any existing jobs for this library
	result := tx.Where("library_id = ? AND status IN ?", libraryID, []string{"running", "pending"}).
		Delete(&database.ScanJob{})
	return result.Error
}

func (s *SafeguardSystem) validateScanStartSuccess(jobID uint32) error {
	// Add validation logic here
	return nil
}

func (s *SafeguardSystem) validatePauseSuccess(jobID uint32) error {
	// Add validation logic here  
	return nil
}

func (s *SafeguardSystem) validateDeletionSuccess(jobID uint32) error {
	// Add validation logic here
	return nil
}

func (s *SafeguardSystem) CleanupLibraryData(libraryID uint32) error {
	// Clean up library data
	return nil
}

func (s *SafeguardSystem) cleanupScanJobData(tx *gorm.DB, scanJobID uint32) error {
	// Clean up scan job data
	return nil
}

func (lm *LockManager) AcquireLibraryLock(libraryID uint32) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	if _, exists := lm.libraryLocks[libraryID]; !exists {
		lm.libraryLocks[libraryID] = &sync.Mutex{}
	}
	
	lm.libraryLocks[libraryID].Lock()
	return nil
}

func (lm *LockManager) ReleaseLibraryLock(libraryID uint32) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	if lock, exists := lm.libraryLocks[libraryID]; exists {
		lock.Unlock()
	}
} 