package host

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// VectorGet retrieves an embedding.
func (s *StorageServer) VectorGet(ctx context.Context, req *pluginv1.VectorQuery) (*pluginv1.VectorGetResponse, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	prefix := sanitizePluginID(pluginID)
	tableName := "plugin_" + prefix + "_embeddings"

	// Check if table exists
	var tableExists int
	if common.IsPostgres(s.dbType) {
		s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1
		`, tableName).Scan(&tableExists)
	} else {
		s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?
		`, tableName).Scan(&tableExists)
	}

	if tableExists == 0 {
		return &pluginv1.VectorGetResponse{Exists: false}, nil
	}

	var text, model sql.NullString
	var dimensions int32

	if common.IsPostgres(s.dbType) {
		var vectorStr string
		err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT vector::text, text, model, dimensions FROM %s WHERE entity_type = $1 AND entity_id = $2
		`, tableName), req.EntityType, req.EntityId).Scan(&vectorStr, &text, &model, &dimensions)
		if err == sql.ErrNoRows {
			return &pluginv1.VectorGetResponse{Exists: false}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("get embedding: %w", err)
		}
		return &pluginv1.VectorGetResponse{
			Exists:     true,
			Vector:     postgresStringToVector(vectorStr),
			Text:       text.String,
			Model:      model.String,
			Dimensions: dimensions,
		}, nil
	}

	// SQLite
	var vectorBytes []byte
	var vectorData interface{}
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT vector, text, model, dimensions FROM %s WHERE entity_type = ? AND entity_id = ?
	`, tableName), req.EntityType, req.EntityId).Scan(&vectorBytes, &text, &model, &vectorData)
	if err == sql.ErrNoRows {
		return &pluginv1.VectorGetResponse{Exists: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get embedding: %w", err)
	}

	vector := bytesToVector(vectorBytes)

	return &pluginv1.VectorGetResponse{
		Exists:     true,
		Vector:     vector,
		Text:       text.String,
		Model:      model.String,
		Dimensions: int32(len(vector)),
	}, nil
}

// VectorDelete removes an embedding.
func (s *StorageServer) VectorDelete(ctx context.Context, req *pluginv1.VectorQuery) (*pluginv1.Empty, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	prefix := sanitizePluginID(pluginID)
	tableName := "plugin_" + prefix + "_embeddings"
	vecTableName := "plugin_" + prefix + "_vec_embeddings"

	if common.IsPostgres(s.dbType) {
		s.db.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s WHERE entity_type = $1 AND entity_id = $2
		`, tableName), req.EntityType, req.EntityId)
	} else {
		// Get embedding ID first for vec0 cleanup
		var embeddingID int64
		s.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT id FROM %s WHERE entity_type = ? AND entity_id = ?
		`, tableName), req.EntityType, req.EntityId).Scan(&embeddingID)

		if embeddingID > 0 {
			s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE embedding_id = ?`, vecTableName), embeddingID)
		}

		s.db.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s WHERE entity_type = ? AND entity_id = ?
		`, tableName), req.EntityType, req.EntityId)
	}

	return &pluginv1.Empty{}, nil
}

// VectorDeleteByType removes all embeddings for an entity type.
func (s *StorageServer) VectorDeleteByType(ctx context.Context, req *pluginv1.VectorTypeQuery) (*pluginv1.VectorDeleteResponse, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	prefix := sanitizePluginID(pluginID)
	tableName := "plugin_" + prefix + "_embeddings"
	vecTableName := "plugin_" + prefix + "_vec_embeddings"

	var result sql.Result
	var err error

	if common.IsPostgres(s.dbType) {
		if req.EntityType == "" {
			result, err = s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, tableName))
		} else {
			result, err = s.db.ExecContext(ctx, fmt.Sprintf(`
				DELETE FROM %s WHERE entity_type = $1
			`, tableName), req.EntityType)
		}
	} else {
		// Clean up vec0 table first
		if req.EntityType == "" {
			s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, vecTableName))
			result, err = s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, tableName))
		} else {
			// Get IDs to delete from vec0
			rows, _ := s.db.QueryContext(ctx, fmt.Sprintf(`
				SELECT id FROM %s WHERE entity_type = ?
			`, tableName), req.EntityType)
			if rows != nil {
				var ids []int64
				for rows.Next() {
					var id int64
					rows.Scan(&id)
					ids = append(ids, id)
				}
				rows.Close()
				for _, id := range ids {
					s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE embedding_id = ?`, vecTableName), id)
				}
			}

			result, err = s.db.ExecContext(ctx, fmt.Sprintf(`
				DELETE FROM %s WHERE entity_type = ?
			`, tableName), req.EntityType)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("delete embeddings: %w", err)
	}

	count, _ := result.RowsAffected()
	return &pluginv1.VectorDeleteResponse{DeletedCount: count}, nil
}

// VectorCount returns the number of embeddings.
func (s *StorageServer) VectorCount(ctx context.Context, req *pluginv1.VectorTypeQuery) (*pluginv1.VectorCountResponse, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	prefix := sanitizePluginID(pluginID)
	tableName := "plugin_" + prefix + "_embeddings"

	// Check if table exists
	var tableExists int
	if common.IsPostgres(s.dbType) {
		s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1
		`, tableName).Scan(&tableExists)
	} else {
		s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?
		`, tableName).Scan(&tableExists)
	}

	if tableExists == 0 {
		return &pluginv1.VectorCountResponse{Count: 0}, nil
	}

	var count int64
	if req.EntityType == "" {
		if common.IsPostgres(s.dbType) {
			s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)).Scan(&count)
		} else {
			s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)).Scan(&count)
		}
	} else {
		if common.IsPostgres(s.dbType) {
			s.db.QueryRowContext(ctx, fmt.Sprintf(`
				SELECT COUNT(*) FROM %s WHERE entity_type = $1
			`, tableName), req.EntityType).Scan(&count)
		} else {
			s.db.QueryRowContext(ctx, fmt.Sprintf(`
				SELECT COUNT(*) FROM %s WHERE entity_type = ?
			`, tableName), req.EntityType).Scan(&count)
		}
	}

	return &pluginv1.VectorCountResponse{Count: count}, nil
}
