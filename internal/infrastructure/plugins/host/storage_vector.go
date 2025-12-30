package host

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	_ "github.com/mattn/go-sqlite3" // SQLite driver (required for sqlite-vec)
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

// VectorSearch performs similarity search.
func (s *StorageServer) VectorSearch(ctx context.Context, req *pluginv1.VectorSearchRequest) (*pluginv1.VectorSearchResponse, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if len(req.QueryVector) == 0 {
		return nil, errors.New("query vector is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	s.logger.Debug("VectorSearch called",
		"plugin_id", pluginID,
		"dimensions", len(req.QueryVector),
		"limit", limit)

	prefix := sanitizePluginID(pluginID)

	if common.IsPostgres(s.dbType) {
		return s.vectorSearchPostgres(ctx, prefix, req, limit)
	}
	return s.vectorSearchSQLite(ctx, prefix, req, limit)
}

func (s *StorageServer) vectorSearchPostgres(ctx context.Context, prefix string, req *pluginv1.VectorSearchRequest, limit int) (*pluginv1.VectorSearchResponse, error) {
	tableName := "plugin_" + prefix + "_embeddings"
	vectorStr := vectorToPostgresString(req.QueryVector)

	// Check if table exists
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)
	`, tableName).Scan(&exists)
	if err != nil || !exists {
		return &pluginv1.VectorSearchResponse{Results: nil, TotalCount: 0}, nil
	}

	// Build query
	query := fmt.Sprintf(`
		SELECT entity_type, entity_id, text, 1 - (vector <=> $1::vector) as similarity
		FROM %s
	`, tableName)
	args := []interface{}{vectorStr}
	argNum := 2

	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = fmt.Sprintf("$%d", argNum)
			args = append(args, t)
			argNum++
		}
		query += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	if req.MinSimilarity > 0 {
		if len(req.EntityTypes) > 0 {
			query += " AND "
		} else {
			query += " WHERE "
		}
		query += fmt.Sprintf("1 - (vector <=> $1::vector) >= $%d", argNum)
		args = append(args, req.MinSimilarity)
		argNum++
	}

	query += fmt.Sprintf(" ORDER BY vector <=> $1::vector LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, limit, int(req.Offset))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []*pluginv1.VectorSearchResult
	for rows.Next() {
		var r pluginv1.VectorSearchResult
		var text sql.NullString
		if err := rows.Scan(&r.EntityType, &r.EntityId, &text, &r.Similarity); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.Text = text.String
		results = append(results, &r)
	}

	// Get total count
	var count int32
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)
	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		countArgs := make([]interface{}, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			countArgs[i] = t
		}
		countQuery += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
		s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&count)
	} else {
		s.db.QueryRowContext(ctx, countQuery).Scan(&count)
	}

	return &pluginv1.VectorSearchResponse{
		Results:    results,
		TotalCount: count,
	}, nil
}

func (s *StorageServer) vectorSearchSQLite(ctx context.Context, prefix string, req *pluginv1.VectorSearchRequest, limit int) (*pluginv1.VectorSearchResponse, error) {
	tableName := "plugin_" + prefix + "_embeddings"
	vecTableName := "plugin_" + prefix + "_vec_embeddings"

	// Check if vec0 table exists
	var vecTableExists int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, vecTableName).Scan(&vecTableExists)

	if vecTableExists == 0 {
		// Fall back to brute force
		return s.vectorSearchSQLiteBruteForce(ctx, tableName, req, limit)
	}

	// Serialize query vector
	queryBytes, err := sqlite_vec.SerializeFloat32(req.QueryVector)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}

	// Build KNN query
	kValue := limit + int(req.Offset)
	if kValue < 10 {
		kValue = 10
	}

	var query string
	var args []interface{}

	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		args = append(args, queryBytes, kValue)

		query = fmt.Sprintf(`
			SELECT v.embedding_id, v.entity_type, v.distance, e.entity_id, e.text
			FROM %s v
			JOIN %s e ON e.id = v.embedding_id
			WHERE v.entity_type IN (%s)
			  AND v.contents_embedding MATCH ?
			  AND k = ?
			ORDER BY v.distance
		`, vecTableName, tableName, strings.Join(placeholders, ","))
	} else {
		args = []interface{}{queryBytes, kValue}
		query = fmt.Sprintf(`
			SELECT v.embedding_id, v.entity_type, v.distance, e.entity_id, e.text
			FROM %s v
			JOIN %s e ON e.id = v.embedding_id
			WHERE v.contents_embedding MATCH ?
			  AND k = ?
			ORDER BY v.distance
		`, vecTableName, tableName)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Fall back to brute force
		return s.vectorSearchSQLiteBruteForce(ctx, tableName, req, limit)
	}
	defer rows.Close()

	var results []*pluginv1.VectorSearchResult
	resultCount := 0

	for rows.Next() {
		var embeddingID int64
		var entityType string
		var distance float32
		var entityID int64
		var text sql.NullString

		if err := rows.Scan(&embeddingID, &entityType, &distance, &entityID, &text); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}

		resultCount++

		// Skip results before offset
		if resultCount <= int(req.Offset) {
			continue
		}

		// Stop after limit
		if len(results) >= limit {
			continue
		}

		// Convert distance to similarity (cosine distance 0-2 -> similarity 0-1)
		similarity := float32(1.0 - (float64(distance) / 2.0))

		if req.MinSimilarity > 0 && similarity < req.MinSimilarity {
			continue
		}

		results = append(results, &pluginv1.VectorSearchResult{
			EntityType: entityType,
			EntityId:   entityID,
			Similarity: similarity,
			Text:       text.String,
		})
	}

	// Get total count
	var count int32
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)
	s.db.QueryRowContext(ctx, countQuery).Scan(&count)

	return &pluginv1.VectorSearchResponse{
		Results:    results,
		TotalCount: count,
	}, nil
}

func (s *StorageServer) vectorSearchSQLiteBruteForce(ctx context.Context, tableName string, req *pluginv1.VectorSearchRequest, limit int) (*pluginv1.VectorSearchResponse, error) {
	// Check if table exists
	var tableExists int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&tableExists)
	if tableExists == 0 {
		return &pluginv1.VectorSearchResponse{Results: nil, TotalCount: 0}, nil
	}

	// Load all embeddings and compute similarity in Go
	query := fmt.Sprintf(`SELECT entity_type, entity_id, vector, text FROM %s`, tableName)
	var args []interface{}

	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		query += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer rows.Close()

	type scoredResult struct {
		result     *pluginv1.VectorSearchResult
		similarity float32
	}

	var scored []scoredResult
	for rows.Next() {
		var entityType string
		var entityID int64
		var vectorBytes []byte
		var text sql.NullString

		if err := rows.Scan(&entityType, &entityID, &vectorBytes, &text); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		vector := bytesToVector(vectorBytes)
		similarity := cosineSimilarity(req.QueryVector, vector)

		if req.MinSimilarity > 0 && similarity < req.MinSimilarity {
			continue
		}

		scored = append(scored, scoredResult{
			result: &pluginv1.VectorSearchResult{
				EntityType: entityType,
				EntityId:   entityID,
				Similarity: similarity,
				Text:       text.String,
			},
			similarity: similarity,
		})
	}

	// Sort by similarity descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].similarity > scored[i].similarity {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Apply offset and limit
	start := int(req.Offset)
	if start > len(scored) {
		start = len(scored)
	}
	end := start + limit
	if end > len(scored) {
		end = len(scored)
	}

	results := make([]*pluginv1.VectorSearchResult, 0, end-start)
	for _, sc := range scored[start:end] {
		results = append(results, sc.result)
	}

	return &pluginv1.VectorSearchResponse{
		Results:    results,
		TotalCount: int32(len(scored)),
	}, nil
}

// VectorSearchText performs text-based search on embedding text field.
func (s *StorageServer) VectorSearchText(ctx context.Context, req *pluginv1.VectorTextSearchRequest) (*pluginv1.VectorSearchResponse, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Query == "" {
		return nil, errors.New("query is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	s.logger.Debug("VectorSearchText called",
		"plugin_id", pluginID,
		"query", req.Query,
		"limit", limit)

	prefix := sanitizePluginID(pluginID)

	if common.IsPostgres(s.dbType) {
		return s.vectorSearchTextPostgres(ctx, prefix, req, limit)
	}
	return s.vectorSearchTextSQLite(ctx, prefix, req, limit)
}

func (s *StorageServer) vectorSearchTextPostgres(ctx context.Context, prefix string, req *pluginv1.VectorTextSearchRequest, limit int) (*pluginv1.VectorSearchResponse, error) {
	tableName := "plugin_" + prefix + "_embeddings"

	// Check if table exists
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)
	`, tableName).Scan(&exists)
	if err != nil || !exists {
		return &pluginv1.VectorSearchResponse{Results: nil, TotalCount: 0}, nil
	}

	// Build query with ILIKE for case-insensitive text search
	query := fmt.Sprintf(`
		SELECT entity_type, entity_id, text
		FROM %s
		WHERE text ILIKE $1
	`, tableName)
	args := []interface{}{"%" + req.Query + "%"}
	argNum := 2

	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = fmt.Sprintf("$%d", argNum)
			args = append(args, t)
			argNum++
		}
		query += fmt.Sprintf(" AND entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	query += fmt.Sprintf(" LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("text search query: %w", err)
	}
	defer rows.Close()

	var results []*pluginv1.VectorSearchResult
	for rows.Next() {
		var r pluginv1.VectorSearchResult
		var text sql.NullString
		if err := rows.Scan(&r.EntityType, &r.EntityId, &text); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.Text = text.String
		r.Similarity = 1.0 // Text match - give high similarity score
		results = append(results, &r)
	}

	return &pluginv1.VectorSearchResponse{
		Results:    results,
		TotalCount: int32(len(results)),
	}, nil
}

func (s *StorageServer) vectorSearchTextSQLite(ctx context.Context, prefix string, req *pluginv1.VectorTextSearchRequest, limit int) (*pluginv1.VectorSearchResponse, error) {
	tableName := "plugin_" + prefix + "_embeddings"

	// Check if table exists
	var tableExists int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&tableExists)
	if tableExists == 0 {
		return &pluginv1.VectorSearchResponse{Results: nil, TotalCount: 0}, nil
	}

	// Build query with LIKE for text search
	query := fmt.Sprintf(`
		SELECT entity_type, entity_id, text
		FROM %s
		WHERE text LIKE ?
	`, tableName)
	args := []interface{}{"%" + req.Query + "%"}

	if len(req.EntityTypes) > 0 {
		placeholders := make([]string, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		query += fmt.Sprintf(" AND entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("text search query: %w", err)
	}
	defer rows.Close()

	var results []*pluginv1.VectorSearchResult
	for rows.Next() {
		var entityType string
		var entityID int64
		var text sql.NullString
		if err := rows.Scan(&entityType, &entityID, &text); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, &pluginv1.VectorSearchResult{
			EntityType: entityType,
			EntityId:   entityID,
			Similarity: 1.0,
			Text:       text.String,
		})
	}

	return &pluginv1.VectorSearchResponse{
		Results:    results,
		TotalCount: int32(len(results)),
	}, nil
}

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

// Helper functions

func (s *StorageServer) ensureVectorTablePostgres(ctx context.Context, tableName string, dimensions int) error {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)
	`, tableName).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = s.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id SERIAL PRIMARY KEY,
			entity_type TEXT NOT NULL,
			entity_id BIGINT NOT NULL,
			vector vector(%d) NOT NULL,
			text TEXT,
			model TEXT,
			dimensions INTEGER NOT NULL DEFAULT %d,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(entity_type, entity_id)
		)
	`, tableName, dimensions, dimensions))
	if err != nil {
		return fmt.Errorf("create embeddings table: %w", err)
	}

	_, err = s.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX %s_vector_idx ON %s USING hnsw (vector vector_cosine_ops)
		WITH (m = 16, ef_construction = 64)
	`, tableName, tableName))
	if err != nil {
		s.logger.Warn("failed to create HNSW index", "error", err)
	}

	return nil
}

func (s *StorageServer) ensureVectorTableSQLite(ctx context.Context, tableName, vecTableName string, dimensions int) error {
	var tableExists int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&tableExists)

	if tableExists == 0 {
		_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
			CREATE TABLE %s (
				id INTEGER PRIMARY KEY,
				entity_type TEXT NOT NULL,
				entity_id INTEGER NOT NULL,
				vector BLOB NOT NULL,
				text TEXT,
				model TEXT,
				dimensions INTEGER NOT NULL DEFAULT %d,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(entity_type, entity_id)
			)
		`, tableName, dimensions))
		if err != nil {
			return fmt.Errorf("create embeddings table: %w", err)
		}

		s.db.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX %s_entity_idx ON %s(entity_type, entity_id)`, tableName, tableName))
	}

	var vecExists int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, vecTableName).Scan(&vecExists)

	if vecExists == 0 {
		_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
			CREATE VIRTUAL TABLE %s USING vec0(
				embedding_id INTEGER PRIMARY KEY,
				entity_type TEXT,
				contents_embedding float[%d] distance_metric=cosine
			)
		`, vecTableName, dimensions))
		if err != nil {
			s.logger.Warn("failed to create vec0 table", "error", err)
		}
	}

	return nil
}

func vectorToPostgresString(v []float32) string {
	strs := make([]string, len(v))
	for i, f := range v {
		strs[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(strs, ",") + "]"
}

func postgresStringToVector(s string) []float32 {
	s = strings.Trim(s, "[]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	v := make([]float32, len(parts))
	for i, p := range parts {
		var f float64
		fmt.Sscanf(strings.TrimSpace(p), "%f", &f)
		v[i] = float32(f)
	}
	return v
}

func vectorToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToVector(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
