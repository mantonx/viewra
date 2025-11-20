package library

import "context"

// CreateLibraryExecutor defines the interface for creating libraries
type CreateLibraryExecutor interface {
	Execute(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error)
}

// UpdateLibraryExecutor defines the interface for updating libraries
type UpdateLibraryExecutor interface {
	Execute(ctx context.Context, id int64, req UpdateLibraryRequest) (LibraryResponse, error)
}

// DeleteLibraryExecutor defines the interface for deleting libraries
type DeleteLibraryExecutor interface {
	Execute(ctx context.Context, id int64) error
}

// GetLibraryExecutor defines the interface for getting a single library
type GetLibraryExecutor interface {
	Execute(ctx context.Context, id int64) (LibraryResponse, error)
}

// ListLibrariesExecutor defines the interface for listing all libraries
type ListLibrariesExecutor interface {
	Execute(ctx context.Context) (ListLibrariesResponse, error)
}

// ScanLibraryExecutor defines the interface for library scanning operations
type ScanLibraryExecutor interface {
	StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error)
	GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error)
	GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error)
	GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error)
}
