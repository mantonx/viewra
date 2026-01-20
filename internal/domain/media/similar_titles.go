package media

// SimilarTitlesRepository defines the interface for similar titles persistence.
type SimilarTitlesRepository interface {
	// Get all similar titles for an entity
	GetSimilarTitles(mediaType string, entityID int64) ([]string, error)

	// Replace all similar titles for an entity (clear + add)
	ReplaceSimilarTitles(mediaType string, entityID int64, titles []string) error

	// Clear all similar titles for an entity
	ClearSimilarTitles(mediaType string, entityID int64) error
}
