// Package builtin contains built-in enrichers that run in-process.
// These enrichers handle local file operations (NFO parsing, local images)
// and can directly import ViewRA's internal libraries.
package builtin

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// NFOEnricher reads metadata from NFO files (Kodi/Plex format).
// Supports movies, TV episodes, and TV shows.
type NFOEnricher struct{}

// NewNFOEnricher creates a new NFO enricher.
func NewNFOEnricher() *NFOEnricher {
	return &NFOEnricher{}
}

// Stage returns the stage name for this enricher.
func (e *NFOEnricher) Stage() string {
	return "nfo"
}

// Metadata returns display information about this enricher.
func (e *NFOEnricher) Metadata() (name, version string) {
	return "NFO Parser", "1.0.0"
}

// Capabilities returns what this enricher provides.
func (e *NFOEnricher) Capabilities() appenrich.EnricherCapabilities {
	return appenrich.NewCapabilitiesBuilder().
		WithMediaTypes(enrichment.MediaTypeMovie, enrichment.MediaTypeTV, enrichment.MediaTypeTVShow).
		WithProvides("metadata", "external_ids").
		AsLocal().
		Build()
}

// Enrich reads NFO metadata for a media item.
func (e *NFOEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	switch req.MediaType {
	case string(enrichment.MediaTypeMovie):
		return e.enrichMovie(ctx, req)
	case string(enrichment.MediaTypeTV):
		return e.enrichTVEpisode(ctx, req)
	case string(enrichment.MediaTypeTVShow):
		return e.enrichTVShow(ctx, req)
	default:
		return appenrich.Skip(fmt.Sprintf("unsupported media type: %s", req.MediaType)), nil
	}
}

// enrichMovie processes NFO for a movie.
func (e *NFOEnricher) enrichMovie(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	// Find NFO file for this movie
	nfoPath, err := nfo.FindMovieNFO(req.FilePath)
	if err != nil || nfoPath == "" {
		return appenrich.Skip("no NFO file found"), nil
	}

	// Parse the NFO file
	metadata, err := nfo.ParseMovieNFO(nfoPath)
	if err != nil {
		// Handle non-retryable errors gracefully
		if errors.Is(err, nfo.ErrEmptyNFO) {
			return appenrich.Skip("NFO file is empty"), nil
		}
		var wrongType nfo.ErrWrongNFOType
		if errors.As(err, &wrongType) {
			return appenrich.Skip(fmt.Sprintf("NFO contains wrong type: %s", wrongType.Actual)), nil
		}
		return nil, fmt.Errorf("parse movie NFO: %w", err)
	}

	// Build response
	resp := appenrich.Match()
	resp.Confidence = 1.0 // NFO files are authoritative

	// Convert metadata to enriched format
	resp.Metadata = &pluginv1.EnrichedMetadata{}

	if metadata.Title != "" {
		resp.Metadata.Title = appenrich.Ptr(metadata.Title)
	}
	if metadata.OriginalTitle != "" {
		resp.Metadata.OriginalTitle = appenrich.Ptr(metadata.OriginalTitle)
	}
	if metadata.SortTitle != "" {
		resp.Metadata.SortTitle = appenrich.Ptr(metadata.SortTitle)
	}
	if metadata.Year > 0 {
		resp.Metadata.Year = appenrich.Ptr(int32(metadata.Year))
	}
	if metadata.Plot != "" {
		resp.Metadata.Plot = appenrich.Ptr(metadata.Plot)
	}
	if metadata.Tagline != "" {
		resp.Metadata.Tagline = appenrich.Ptr(metadata.Tagline)
	}
	if metadata.ContentRating != "" {
		resp.Metadata.ContentRating = appenrich.Ptr(metadata.ContentRating)
	}
	if metadata.RuntimeMinutes > 0 {
		resp.Metadata.RuntimeMinutes = appenrich.Ptr(int32(metadata.RuntimeMinutes))
	}
	if metadata.Director != "" {
		resp.Metadata.Directors = []string{metadata.Director}
	}
	if len(metadata.Cast) > 0 {
		for i, name := range metadata.Cast {
			resp.Metadata.Cast = append(resp.Metadata.Cast, &pluginv1.CastMember{
				Name:  name,
				Order: int32(i),
			})
		}
	}
	if len(metadata.Genre) > 0 {
		resp.Metadata.Genres = metadata.Genre
	}

	// Store discovered external IDs
	if metadata.IMDbID != "" {
		appenrich.AddDiscoveredID(resp, "imdb", metadata.IMDbID)
	}
	if metadata.TMDbID > 0 {
		appenrich.AddDiscoveredID(resp, "tmdb", strconv.Itoa(metadata.TMDbID))
	}

	return resp, nil
}

// enrichTVEpisode processes NFO for a TV episode.
func (e *NFOEnricher) enrichTVEpisode(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	// Find NFO file for this episode
	nfoPath, err := nfo.FindEpisodeNFO(req.FilePath)
	if err != nil || nfoPath == "" {
		return appenrich.Skip("no NFO file found"), nil
	}

	// Parse the NFO file
	metadata, err := nfo.ParseEpisodeNFO(nfoPath)
	if err != nil {
		// Handle non-retryable errors gracefully
		if errors.Is(err, nfo.ErrEmptyNFO) {
			return appenrich.Skip("NFO file is empty"), nil
		}
		var wrongType nfo.ErrWrongNFOType
		if errors.As(err, &wrongType) {
			return appenrich.Skip(fmt.Sprintf("NFO contains wrong type: %s", wrongType.Actual)), nil
		}
		return nil, fmt.Errorf("parse episode NFO: %w", err)
	}

	// Build response
	resp := appenrich.Match()
	resp.Confidence = 1.0 // NFO files are authoritative

	// Convert metadata to enriched format
	resp.Metadata = &pluginv1.EnrichedMetadata{}

	if metadata.Title != "" {
		resp.Metadata.Title = appenrich.Ptr(metadata.Title)
	}
	if metadata.Plot != "" {
		resp.Metadata.Plot = appenrich.Ptr(metadata.Plot)
	}
	if !metadata.AirDate.IsZero() {
		resp.Metadata.Premiered = appenrich.Ptr(metadata.AirDate.Format("2006-01-02"))
	}
	if metadata.RuntimeMinutes > 0 {
		resp.Metadata.RuntimeMinutes = appenrich.Ptr(int32(metadata.RuntimeMinutes))
	}

	// Store discovered external IDs
	if metadata.IMDbID != "" {
		appenrich.AddDiscoveredID(resp, "imdb", metadata.IMDbID)
	}
	if metadata.TVDbID > 0 {
		appenrich.AddDiscoveredID(resp, "tvdb", strconv.Itoa(metadata.TVDbID))
	}

	return resp, nil
}

// enrichTVShow processes NFO for a TV show (parent entity).
func (e *NFOEnricher) enrichTVShow(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	// FilePath contains the show directory for TV shows
	nfoPath, err := nfo.FindTVShowNFO(req.FilePath)
	if err != nil || nfoPath == "" {
		return appenrich.Skip("no NFO file found"), nil
	}

	// Parse the NFO file
	metadata, err := nfo.ParseTVShowNFO(nfoPath)
	if err != nil {
		// Handle non-retryable errors gracefully
		if errors.Is(err, nfo.ErrEmptyNFO) {
			return appenrich.Skip("NFO file is empty"), nil
		}
		var wrongType nfo.ErrWrongNFOType
		if errors.As(err, &wrongType) {
			return appenrich.Skip(fmt.Sprintf("NFO contains wrong type: %s", wrongType.Actual)), nil
		}
		return nil, fmt.Errorf("parse tvshow NFO: %w", err)
	}

	// Build response
	resp := appenrich.Match()
	resp.Confidence = 1.0 // NFO files are authoritative

	// Convert metadata to enriched format
	resp.Metadata = &pluginv1.EnrichedMetadata{}

	if metadata.Title != "" {
		resp.Metadata.Title = appenrich.Ptr(metadata.Title)
	}
	if metadata.OriginalTitle != "" {
		resp.Metadata.OriginalTitle = appenrich.Ptr(metadata.OriginalTitle)
	}
	if metadata.SortTitle != "" {
		resp.Metadata.SortTitle = appenrich.Ptr(metadata.SortTitle)
	}
	if metadata.Year > 0 {
		resp.Metadata.Year = appenrich.Ptr(int32(metadata.Year))
	}
	if metadata.Plot != "" {
		resp.Metadata.Plot = appenrich.Ptr(metadata.Plot)
	}
	if metadata.Tagline != "" {
		resp.Metadata.Tagline = appenrich.Ptr(metadata.Tagline)
	}
	if metadata.ContentRating != "" {
		resp.Metadata.ContentRating = appenrich.Ptr(metadata.ContentRating)
	}
	if !metadata.PremiereDate.IsZero() {
		resp.Metadata.Premiered = appenrich.Ptr(metadata.PremiereDate.Format("2006-01-02"))
	}
	if len(metadata.Cast) > 0 {
		for i, name := range metadata.Cast {
			resp.Metadata.Cast = append(resp.Metadata.Cast, &pluginv1.CastMember{
				Name:  name,
				Order: int32(i),
			})
		}
	}
	if len(metadata.Genre) > 0 {
		resp.Metadata.Genres = metadata.Genre
	}
	if metadata.Network != "" {
		resp.Metadata.Studios = []string{metadata.Network}
	}

	// Store discovered external IDs
	if metadata.IMDbID != "" {
		appenrich.AddDiscoveredID(resp, "imdb", metadata.IMDbID)
	}
	if metadata.TVDbID > 0 {
		appenrich.AddDiscoveredID(resp, "tvdb", strconv.Itoa(metadata.TVDbID))
	}
	if metadata.TMDbID > 0 {
		appenrich.AddDiscoveredID(resp, "tmdb", strconv.Itoa(metadata.TMDbID))
	}

	return resp, nil
}
