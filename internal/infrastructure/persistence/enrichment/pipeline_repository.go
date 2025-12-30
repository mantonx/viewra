package enrichment

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewPipelineRepository creates a new pipeline repository with the appropriate database driver.
func NewPipelineRepository(db *sql.DB, driver string) *PipelineRepository {
	r := &PipelineRepository{
		db:     db,
		dbType: driver,
		router: common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// Create adds a new stage to a pipeline.
func (r *PipelineRepository) Create(ctx context.Context, stage *enrichment.PipelineStage) (*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreatePipelineStage(ctx, sqlc_postgres.CreatePipelineStageParams{
				MediaType:  string(stage.MediaType),
				PluginID:   stage.PluginID,
				StageName:  stage.StageName,
				Position:   int64(stage.Position),
				Enabled:    common.NullInt64FromBool(stage.Enabled),
				ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
			})
		},
		func() (any, error) {
			return r.sqlite.CreatePipelineStage(ctx, sqlc_sqlite.CreatePipelineStageParams{
				MediaType:  string(stage.MediaType),
				PluginID:   stage.PluginID,
				StageName:  stage.StageName,
				Position:   int64(stage.Position),
				Enabled:    common.NullInt64FromBool(stage.Enabled),
				ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToPipelineStage(result), nil
}

// GetEnabledStages returns all enabled stages for a media type, ordered by position.
func (r *PipelineRepository) GetEnabledStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEnabledPipelineStages(ctx, string(mediaType))
		},
		func() (any, error) {
			return r.sqlite.GetEnabledPipelineStages(ctx, string(mediaType))
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgStages := result.([]sqlc_postgres.EnrichmentPipeline)
		stages := make([]*enrichment.PipelineStage, len(pgStages))
		for i, pgStage := range pgStages {
			stages[i] = r.convertToPipelineStage(pgStage)
		}
		return stages, nil
	}

	sqStages := result.([]sqlc_sqlite.EnrichmentPipeline)
	stages := make([]*enrichment.PipelineStage, len(sqStages))
	for i, sqStage := range sqStages {
		stages[i] = r.convertToPipelineStage(sqStage)
	}
	return stages, nil
}

// GetAllStages returns all stages for a media type, including disabled.
func (r *PipelineRepository) GetAllStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetAllPipelineStages(ctx, string(mediaType))
		},
		func() (any, error) {
			return r.sqlite.GetAllPipelineStages(ctx, string(mediaType))
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgStages := result.([]sqlc_postgres.EnrichmentPipeline)
		stages := make([]*enrichment.PipelineStage, len(pgStages))
		for i, pgStage := range pgStages {
			stages[i] = r.convertToPipelineStage(pgStage)
		}
		return stages, nil
	}

	sqStages := result.([]sqlc_sqlite.EnrichmentPipeline)
	stages := make([]*enrichment.PipelineStage, len(sqStages))
	for i, sqStage := range sqStages {
		stages[i] = r.convertToPipelineStage(sqStage)
	}
	return stages, nil
}

// GetFirstStage returns the first enabled stage for a media type.
func (r *PipelineRepository) GetFirstStage(ctx context.Context, mediaType enrichment.MediaType) (*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetFirstPipelineStage(ctx, string(mediaType))
		},
		func() (any, error) {
			return r.sqlite.GetFirstPipelineStage(ctx, string(mediaType))
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToPipelineStage(result), nil
}

// GetNextStage returns the next enabled stage after the given position.
func (r *PipelineRepository) GetNextStage(ctx context.Context, mediaType enrichment.MediaType, currentPosition int) (*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetNextPipelineStage(ctx, sqlc_postgres.GetNextPipelineStageParams{
				MediaType: string(mediaType),
				Position:  int64(currentPosition),
			})
		},
		func() (any, error) {
			return r.sqlite.GetNextPipelineStage(ctx, sqlc_sqlite.GetNextPipelineStageParams{
				MediaType: string(mediaType),
				Position:  int64(currentPosition),
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No more stages
		}
		return nil, err
	}

	return r.convertToPipelineStage(result), nil
}

// GetStageByName returns a stage by media type and stage name.
func (r *PipelineRepository) GetStageByName(ctx context.Context, mediaType enrichment.MediaType, stageName string) (*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetPipelineStageByName(ctx, sqlc_postgres.GetPipelineStageByNameParams{
				MediaType: string(mediaType),
				StageName: stageName,
			})
		},
		func() (any, error) {
			return r.sqlite.GetPipelineStageByName(ctx, sqlc_sqlite.GetPipelineStageByNameParams{
				MediaType: string(mediaType),
				StageName: stageName,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Stage not found
		}
		return nil, err
	}

	return r.convertToPipelineStage(result), nil
}

// Update modifies a pipeline stage.
func (r *PipelineRepository) Update(ctx context.Context, stage *enrichment.PipelineStage) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.UpdatePipelineStage(ctx, sqlc_postgres.UpdatePipelineStageParams{
				StageName:  stage.StageName,
				Position:   int64(stage.Position),
				Enabled:    common.NullInt64FromBool(stage.Enabled),
				ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
				ID:         stage.ID,
			})
		},
		func() error {
			return r.sqlite.UpdatePipelineStage(ctx, sqlc_sqlite.UpdatePipelineStageParams{
				StageName:  stage.StageName,
				Position:   int64(stage.Position),
				Enabled:    common.NullInt64FromBool(stage.Enabled),
				ConfigJson: sql.NullString{String: stage.ConfigJSON, Valid: stage.ConfigJSON != ""},
				ID:         stage.ID,
			})
		},
	)
}

// Delete removes a pipeline stage.
func (r *PipelineRepository) Delete(ctx context.Context, stageID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeletePipelineStage(ctx, stageID)
		},
		func() error {
			return r.sqlite.DeletePipelineStage(ctx, stageID)
		},
	)
}

// Enable toggles a stage on.
func (r *PipelineRepository) Enable(ctx context.Context, stageID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.EnablePipelineStage(ctx, stageID)
		},
		func() error {
			return r.sqlite.EnablePipelineStage(ctx, stageID)
		},
	)
}

// Disable toggles a stage off.
func (r *PipelineRepository) Disable(ctx context.Context, stageID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DisablePipelineStage(ctx, stageID)
		},
		func() error {
			return r.sqlite.DisablePipelineStage(ctx, stageID)
		},
	)
}

// GetByID retrieves a pipeline stage by ID.
func (r *PipelineRepository) GetByID(ctx context.Context, stageID int64) (*enrichment.PipelineStage, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetPipelineStage(ctx, stageID)
		},
		func() (any, error) {
			return r.sqlite.GetPipelineStage(ctx, stageID)
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToPipelineStage(result), nil
}

// convertToPipelineStage converts sqlc result to domain PipelineStage.
func (r *PipelineRepository) convertToPipelineStage(result any) *enrichment.PipelineStage {
	if r.router.IsPostgresDB() {
		pgStage := result.(sqlc_postgres.EnrichmentPipeline)
		return &enrichment.PipelineStage{
			ID:         pgStage.ID,
			MediaType:  enrichment.MediaType(pgStage.MediaType),
			PluginID:   pgStage.PluginID,
			StageName:  pgStage.StageName,
			Position:   int(pgStage.Position),
			Enabled:    pgStage.Enabled.Int64 == 1,
			ConfigJSON: common.ParseNullString(pgStage.ConfigJson),
			CreatedAt:  common.ParseNullTime(pgStage.CreatedAt),
			UpdatedAt:  common.ParseNullTime(pgStage.UpdatedAt),
		}
	}

	sqStage := result.(sqlc_sqlite.EnrichmentPipeline)
	return &enrichment.PipelineStage{
		ID:         sqStage.ID,
		MediaType:  enrichment.MediaType(sqStage.MediaType),
		PluginID:   sqStage.PluginID,
		StageName:  sqStage.StageName,
		Position:   int(sqStage.Position),
		Enabled:    sqStage.Enabled.Int64 == 1,
		ConfigJSON: common.ParseNullString(sqStage.ConfigJson),
		CreatedAt:  common.ParseNullTime(sqStage.CreatedAt),
		UpdatedAt:  common.ParseNullTime(sqStage.UpdatedAt),
	}
}
