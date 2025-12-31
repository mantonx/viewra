package enrichment

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewPipelineRepository creates a new pipeline repository with the appropriate database driver.
func NewPipelineRepository(db *common.BaseRepository) *PipelineRepository {
	return &PipelineRepository{
		BaseRepository: db,
	}
}

// Create adds a new stage to a pipeline.
func (r *PipelineRepository) Create(ctx context.Context, stage *enrichment.PipelineStage) (*enrichment.PipelineStage, error) {
	row, err := r.Q().CreatePipelineStage(ctx, unified.CreatePipelineStageParams{
		MediaType:  string(stage.MediaType),
		PluginID:   stage.PluginID,
		StageName:  stage.StageName,
		Position:   int64(stage.Position),
		Enabled:    common.NullInt64FromBool(stage.Enabled),
		ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
	})
	if err != nil {
		return nil, err
	}
	return convertPipelineStage(row), nil
}

// GetEnabledStages returns all enabled stages for a media type, ordered by position.
func (r *PipelineRepository) GetEnabledStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	rows, err := r.Q().GetEnabledPipelineStages(ctx, string(mediaType))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertPipelineStage), nil
}

// GetAllStages returns all stages for a media type, including disabled.
func (r *PipelineRepository) GetAllStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	rows, err := r.Q().GetAllPipelineStages(ctx, string(mediaType))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertPipelineStage), nil
}

// GetFirstStage returns the first enabled stage for a media type.
func (r *PipelineRepository) GetFirstStage(ctx context.Context, mediaType enrichment.MediaType) (*enrichment.PipelineStage, error) {
	row, err := r.Q().GetFirstPipelineStage(ctx, string(mediaType))
	if err != nil {
		return nil, err
	}
	return convertPipelineStage(row), nil
}

// GetNextStage returns the next enabled stage after the given position.
func (r *PipelineRepository) GetNextStage(ctx context.Context, mediaType enrichment.MediaType, currentPosition int) (*enrichment.PipelineStage, error) {
	row, err := r.Q().GetNextPipelineStage(ctx, unified.GetNextPipelineStageParams{
		MediaType: string(mediaType),
		Position:  int64(currentPosition),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No more stages
		}
		return nil, err
	}
	return convertPipelineStage(row), nil
}

// GetStageByName returns a stage by media type and stage name.
func (r *PipelineRepository) GetStageByName(ctx context.Context, mediaType enrichment.MediaType, stageName string) (*enrichment.PipelineStage, error) {
	row, err := r.Q().GetPipelineStageByName(ctx, unified.GetPipelineStageByNameParams{
		MediaType: string(mediaType),
		StageName: stageName,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Stage not found
		}
		return nil, err
	}
	return convertPipelineStage(row), nil
}

// Update modifies a pipeline stage.
func (r *PipelineRepository) Update(ctx context.Context, stage *enrichment.PipelineStage) error {
	return r.Q().UpdatePipelineStage(ctx, unified.UpdatePipelineStageParams{
		StageName:  stage.StageName,
		Position:   int64(stage.Position),
		Enabled:    common.NullInt64FromBool(stage.Enabled),
		ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
		ID:         stage.ID,
	})
}

// Delete removes a pipeline stage.
func (r *PipelineRepository) Delete(ctx context.Context, stageID int64) error {
	return r.Q().DeletePipelineStage(ctx, stageID)
}

// Enable toggles a stage on.
func (r *PipelineRepository) Enable(ctx context.Context, stageID int64) error {
	return r.Q().EnablePipelineStage(ctx, stageID)
}

// Disable toggles a stage off.
func (r *PipelineRepository) Disable(ctx context.Context, stageID int64) error {
	return r.Q().DisablePipelineStage(ctx, stageID)
}

// GetByID retrieves a pipeline stage by ID.
func (r *PipelineRepository) GetByID(ctx context.Context, stageID int64) (*enrichment.PipelineStage, error) {
	row, err := r.Q().GetPipelineStage(ctx, stageID)
	if err != nil {
		return nil, err
	}
	return convertPipelineStage(row), nil
}
