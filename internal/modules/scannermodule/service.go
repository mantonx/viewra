package scannermodule

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/types"
)

// ServiceAdapter adapts the scanner module to implement services.ScannerService
type ServiceAdapter struct {
	module *Module
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(module *Module) *ServiceAdapter {
	return &ServiceAdapter{
		module: module,
	}
}

// StartScan starts a new scan for a library
func (s *ServiceAdapter) StartScan(ctx context.Context, libraryID uint32) (*types.ScanJob, error) {
	if s.module.scannerManager == nil {
		return nil, fmt.Errorf("scanner manager not initialized")
	}

	job, err := s.module.scannerManager.StartScan(libraryID)
	if err != nil {
		return nil, err
	}

	// Convert job ID to string and create a minimal scan job response
	return &types.ScanJob{
		ID:         fmt.Sprintf("%d", job.ID),
		LibraryID:  libraryID,
		Status:     "running",
		Progress:   0.0,
		StartedAt:  time.Now(),
		FilesFound: 0,
		FilesAdded: 0,
	}, nil
}

// GetScanProgress gets the progress of a scan job
func (s *ServiceAdapter) GetScanProgress(ctx context.Context, jobID string) (*types.ScanProgress, error) {
	if s.module.scannerManager == nil {
		return nil, fmt.Errorf("scanner manager not initialized")
	}

	// For now, return a minimal progress response
	// The actual implementation would need to query the scan job from the database
	return &types.ScanProgress{
		JobID:        jobID,
		Progress:     0.0,
		CurrentPath:  "",
		FilesScanned: 0,
		BytesScanned: 0,
		Rate:         0.0,
		ETA:          time.Now(),
	}, nil
}

// StopScan stops a running scan
func (s *ServiceAdapter) StopScan(ctx context.Context, jobID string) error {
	if s.module.scannerManager == nil {
		return fmt.Errorf("scanner manager not initialized")
	}

	// Convert string jobID to uint32
	var jobIDUint uint32
	if _, err := fmt.Sscanf(jobID, "%d", &jobIDUint); err != nil {
		return fmt.Errorf("invalid job ID format: %s", jobID)
	}

	return s.module.scannerManager.StopScan(jobIDUint)
}

// GetActiveScanJobs returns all active scan jobs
func (s *ServiceAdapter) GetActiveScanJobs(ctx context.Context) ([]*types.ScanJob, error) {
	if s.module.scannerManager == nil {
		return nil, fmt.Errorf("scanner manager not initialized")
	}

	// For now, return an empty list
	// The actual implementation would need to query active jobs from the database
	return []*types.ScanJob{}, nil
}

// SetScanInterval sets the scan interval for a library
func (s *ServiceAdapter) SetScanInterval(ctx context.Context, libraryID uint32, interval time.Duration) error {
	if s.module.scannerManager == nil {
		return fmt.Errorf("scanner manager not initialized")
	}

	// This functionality might need to be implemented in the scanner manager
	// For now, return a not implemented error
	return fmt.Errorf("set scan interval not implemented")
}

// GetScanHistory returns the scan history for a library
func (s *ServiceAdapter) GetScanHistory(ctx context.Context, libraryID uint32) ([]*types.ScanResult, error) {
	if s.module.scannerManager == nil {
		return nil, fmt.Errorf("scanner manager not initialized")
	}

	// For now, return an empty list
	// The actual implementation would need to query scan history from the database
	return []*types.ScanResult{}, nil
}
