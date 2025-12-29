package library

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/common"
)

// Repository defines the interface for library data access
type Repository interface {
	// Create creates a new library
	Create(ctx context.Context, library *Library) error

	// GetByID retrieves a library by its ID
	GetByID(ctx context.Context, id int64) (*Library, error)

	// GetByPath retrieves a library by its path
	GetByPath(ctx context.Context, path string) (*Library, error)

	// List retrieves all libraries
	List(ctx context.Context) ([]*Library, error)

	// Update updates an existing library
	Update(ctx context.Context, library *Library) error

	// Delete deletes a library by its ID
	Delete(ctx context.Context, id int64) error

	// Exists checks if a library with the given path exists
	Exists(ctx context.Context, path string) (bool, error)

	// Transaction-aware methods
	CreateWithTx(ctx context.Context, tx common.Transaction, library *Library) error
	GetByIDWithTx(ctx context.Context, tx common.Transaction, id int64) (*Library, error)
	DeleteWithTx(ctx context.Context, tx common.Transaction, id int64) error
	ExistsWithTx(ctx context.Context, tx common.Transaction, path string) (bool, error)

	// Monitoring methods
	UpdateMonitoring(ctx context.Context, id int64, enabled bool, config *MonitoringConfig) error
	ListMonitored(ctx context.Context) ([]*Library, error)
}
