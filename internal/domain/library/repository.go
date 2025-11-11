package library

import "context"

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
}
