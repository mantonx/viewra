package library

import (
	"context"

	"github.com/mantonx/viewra/internal/application/library/scan"
)

// LibraryService defines the interface for library CRUD operations
type LibraryServiceInterface interface {
	Create(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error)
	Get(ctx context.Context, id int64) (LibraryResponse, error)
	List(ctx context.Context) (ListLibrariesResponse, error)
	Update(ctx context.Context, id int64, req UpdateLibraryRequest) (LibraryResponse, error)
	Delete(ctx context.Context, id int64) error
}

// ScanLibraryExecutor defines the interface for library scanning operations
type ScanLibraryExecutor interface {
	StartScan(ctx context.Context, libraryID int64) (scan.StartScanResponse, error)
	StartTargetedScan(ctx context.Context, libraryID int64, targetPaths []string) (interface{}, error)
	ResumeScan(ctx context.Context, jobID int64) error
	GetProgress(ctx context.Context, jobID int64) (scan.ScanProgressResponse, error)
	GetLatestScan(ctx context.Context, libraryID int64) (scan.ScanProgressResponse, error)
	GetScanHistory(ctx context.Context, libraryID int64, limit int32) (scan.ScanHistoryResponse, error)
}
