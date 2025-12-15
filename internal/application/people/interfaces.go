package people

import "context"

// GetPersonExecutor defines the interface for getting a person by ID.
type GetPersonExecutor interface {
	Execute(ctx context.Context, id int64) (*PersonResponse, error)
}

// GetCreditsForEntityExecutor defines the interface for getting credits for a media entity.
type GetCreditsForEntityExecutor interface {
	Execute(ctx context.Context, mediaType string, entityID int64) (*CreditsResponse, error)
}

// GetCreditsForPersonExecutor defines the interface for getting a person's filmography.
type GetCreditsForPersonExecutor interface {
	Execute(ctx context.Context, personID int64) (*PersonCreditsResponse, error)
}
