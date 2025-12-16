package media

// Studio represents a production company or studio.
type Studio struct {
	ID       int64
	Name     string
	LogoPath string
	TMDbID   int
}

// StudioRepository defines the interface for studio persistence.
type StudioRepository interface {
	// Studio CRUD operations
	GetStudioByID(id int64) (*Studio, error)
	GetStudioByName(name string) (*Studio, error)
	GetStudioByTMDbID(tmdbID int) (*Studio, error)
	CreateStudio(studio *Studio) error
	UpdateStudio(studio *Studio) error
	FindOrCreateStudio(name string, tmdbID int) (*Studio, error)

	// Media-studio associations
	GetStudiosForEntity(mediaType string, entityID int64) ([]*Studio, error)
	AddStudioToEntity(mediaType string, entityID int64, studioID int64) error
	RemoveStudioFromEntity(mediaType string, entityID int64, studioID int64) error
	ClearStudiosForEntity(mediaType string, entityID int64) error
	ReplaceStudiosForEntity(mediaType string, entityID int64, studios []*Studio) error
}
