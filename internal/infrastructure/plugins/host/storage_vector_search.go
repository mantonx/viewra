package host

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

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
