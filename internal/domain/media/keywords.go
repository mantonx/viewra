package media

// Keyword represents a tag/keyword associated with media (e.g., from TMDB).
// Keywords can indicate themes, settings, plot elements, etc.
type Keyword struct {
	ID         int64
	KeywordID  int    // External ID (e.g., TMDB keyword ID)
	Name       string // Keyword text
	IsLocation bool   // Whether this keyword represents a location/setting
}

// KeywordRepository defines the interface for keyword persistence.
type KeywordRepository interface {
	// Add or update a keyword for an entity
	UpsertKeyword(mediaType string, entityID int64, keyword *Keyword) error

	// Get all keywords for an entity
	GetKeywordsForEntity(mediaType string, entityID int64) ([]*Keyword, error)

	// Get only location-related keywords for an entity
	GetLocationKeywordsForEntity(mediaType string, entityID int64) ([]*Keyword, error)

	// Clear all keywords for an entity
	ClearKeywordsForEntity(mediaType string, entityID int64) error

	// Replace all keywords for an entity (clear + add)
	ReplaceKeywordsForEntity(mediaType string, entityID int64, keywords []*Keyword) error
}
