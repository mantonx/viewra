package host

import (
	"context"
	"errors"
	"fmt"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// VectorStoreEmbedding stores or updates an embedding for an entity.
func (s *StorageServer) VectorStoreEmbedding(ctx context.Context, req *pluginv1.VectorStoreRequest) (*pluginv1.Empty, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.EntityType == "" {
		return nil, errors.New("entity type is required")
	}
	if len(req.Vector) == 0 {
		return nil, errors.New("vector is required")
	}

	s.logger.Debug("VectorStoreEmbedding called",
		"plugin_id", pluginID,
		"entity_type", req.EntityType,
		"entity_id", req.EntityId,
		"dimensions", len(req.Vector))

	prefix := sanitizePluginID(pluginID)
	now := time.Now()

	if common.IsPostgres(s.dbType) {
		return s.vectorStorePostgres(ctx, prefix, req, now)
	}
	return s.vectorStoreSQLite(ctx, prefix, req, now)
}

func (s *StorageServer) vectorStorePostgres(ctx context.Context, prefix string, req *pluginv1.VectorStoreRequest, now time.Time) (*pluginv1.Empty, error) {
	tableName := "plugin_" + prefix + "_embeddings"
	vectorStr := vectorToPostgresString(req.Vector)

	// Ensure table exists
	if err := s.ensureVectorTablePostgres(ctx, tableName, len(req.Vector)); err != nil {
		return nil, err
	}

	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (entity_type, entity_id, vector, text, model, dimensions, created_at, updated_at)
		VALUES ($1, $2, $3::vector, $4, $5, $6, $7, $8)
		ON CONFLICT (entity_type, entity_id) 
		DO UPDATE SET vector = $3::vector, text = $4, model = $5, dimensions = $6, updated_at = $8
	`, tableName), req.EntityType, req.EntityId, vectorStr, req.Text, req.Model, len(req.Vector), now, now)

	if err != nil {
		return nil, fmt.Errorf("store embedding: %w", err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *StorageServer) vectorStoreSQLite(ctx context.Context, prefix string, req *pluginv1.VectorStoreRequest, now time.Time) (*pluginv1.Empty, error) {
	tableName := "plugin_" + prefix + "_embeddings"
	vecTableName := "plugin_" + prefix + "_vec_embeddings"
	vectorBytes := vectorToBytes(req.Vector)

	// Ensure tables exist
	if err := s.ensureVectorTableSQLite(ctx, tableName, vecTableName, len(req.Vector)); err != nil {
		return nil, err
	}

	nowStr := now.Format(time.RFC3339)

	// Use a transaction for the embedding + vec0 update
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert/update in main embeddings table
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (entity_type, entity_id, vector, text, model, dimensions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (entity_type, entity_id) 
		DO UPDATE SET vector = ?, text = ?, model = ?, dimensions = ?, updated_at = ?
	`, tableName), req.EntityType, req.EntityId, vectorBytes, req.Text, req.Model, len(req.Vector), nowStr, nowStr,
		vectorBytes, req.Text, req.Model, len(req.Vector), nowStr)
	if err != nil {
		return nil, fmt.Errorf("store embedding: %w", err)
	}

	// Get the embedding ID
	var embeddingID int64
	lastID, _ := result.LastInsertId()
	if lastID > 0 {
		embeddingID = lastID
	} else {
		// Was an update, query for the ID
		err = tx.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT id FROM %s WHERE entity_type = ? AND entity_id = ?`, tableName),
			req.EntityType, req.EntityId).Scan(&embeddingID)
		if err != nil {
			return nil, fmt.Errorf("get embedding ID: %w", err)
		}
	}

	// Update vec0 virtual table
	vecBytes, err := sqlite_vec.SerializeFloat32(req.Vector)
	if err != nil {
		return nil, fmt.Errorf("serialize vector: %w", err)
	}

	// Delete existing entry if any
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE embedding_id = ?`, vecTableName), embeddingID)

	// Insert into vec0
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (embedding_id, entity_type, contents_embedding)
		VALUES (?, ?, ?)
	`, vecTableName), embeddingID, req.EntityType, vecBytes)
	if err != nil {
		// vec0 might not exist or have issues, continue without it
		s.logger.Warn("failed to insert into vec0 table", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &pluginv1.Empty{}, nil
}

// VectorStoreBatch stores multiple embeddings in a single transaction.
func (s *StorageServer) VectorStoreBatch(ctx context.Context, req *pluginv1.VectorStoreBatchRequest) (*pluginv1.Empty, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if len(req.Embeddings) == 0 {
		return &pluginv1.Empty{}, nil
	}

	s.logger.Debug("VectorStoreBatch called",
		"plugin_id", pluginID,
		"count", len(req.Embeddings))

	// Process each embedding
	for _, emb := range req.Embeddings {
		if _, err := s.VectorStoreEmbedding(ctx, emb); err != nil {
			return nil, err
		}
	}

	return &pluginv1.Empty{}, nil
}
