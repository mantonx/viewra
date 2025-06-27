package enrichmentmodule

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/types"
)

// ServiceAdapter adapts the enrichment module to implement services.EnrichmentService
type ServiceAdapter struct {
	module *Module
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(module *Module) *ServiceAdapter {
	return &ServiceAdapter{
		module: module,
	}
}

// EnrichMovie enriches a movie with metadata from external sources
func (s *ServiceAdapter) EnrichMovie(ctx context.Context, movieID string) error {
	if s.module == nil || !s.module.enabled {
		return fmt.Errorf("enrichment module not available")
	}

	// Create enrichment job for the movie
	job := EnrichmentJob{
		MediaFileID: movieID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}

	if err := s.module.db.Create(&job).Error; err != nil {
		return fmt.Errorf("failed to create enrichment job: %w", err)
	}

	return nil
}

// EnrichEpisode enriches an episode with metadata from external sources
func (s *ServiceAdapter) EnrichEpisode(ctx context.Context, episodeID string) error {
	if s.module == nil || !s.module.enabled {
		return fmt.Errorf("enrichment module not available")
	}

	// Create enrichment job for the episode
	job := EnrichmentJob{
		MediaFileID: episodeID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}

	if err := s.module.db.Create(&job).Error; err != nil {
		return fmt.Errorf("failed to create enrichment job: %w", err)
	}

	return nil
}

// EnrichSeries enriches a TV series with metadata from external sources
func (s *ServiceAdapter) EnrichSeries(ctx context.Context, seriesID string) error {
	if s.module == nil || !s.module.enabled {
		return fmt.Errorf("enrichment module not available")
	}

	// Create enrichment job for the series
	job := EnrichmentJob{
		MediaFileID: seriesID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}

	if err := s.module.db.Create(&job).Error; err != nil {
		return fmt.Errorf("failed to create enrichment job: %w", err)
	}

	return nil
}

// BatchEnrich enriches multiple media items of the same type
func (s *ServiceAdapter) BatchEnrich(ctx context.Context, mediaType string, ids []string) error {
	if s.module == nil || !s.module.enabled {
		return fmt.Errorf("enrichment module not available")
	}

	// Create enrichment jobs for all IDs
	for _, id := range ids {
		job := EnrichmentJob{
			MediaFileID: id,
			JobType:     "apply_enrichment",
			Status:      "pending",
		}

		if err := s.module.db.Create(&job).Error; err != nil {
			// Log error but continue with other jobs
			continue
		}
	}

	return nil
}

// GetEnrichmentStatus returns the current status of an enrichment job
func (s *ServiceAdapter) GetEnrichmentStatus(ctx context.Context, jobID string) (*types.EnrichmentStatus, error) {
	if s.module == nil || !s.module.enabled {
		return nil, fmt.Errorf("enrichment module not available")
	}

	var job EnrichmentJob
	if err := s.module.db.Where("id = ?", jobID).First(&job).Error; err != nil {
		return nil, fmt.Errorf("enrichment job not found: %w", err)
	}

	// Convert to types.EnrichmentStatus
	status := &types.EnrichmentStatus{
		JobID:     fmt.Sprintf("%d", job.ID),
		MediaType: "", // Would need to look up from media file
		MediaIDs:  []string{job.MediaFileID},
		Status:    job.Status,
		Progress:  0.0, // Would need to calculate based on job type
		StartedAt: job.CreatedAt,
	}

	if job.Status == "completed" || job.Status == "failed" {
		status.CompletedAt = &job.UpdatedAt
	}

	return status, nil
}
