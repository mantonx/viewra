// Package ai provides persistence for AI-related data.
package ai

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/mantonx/viewra/internal/domain/ai"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// EmbeddingRepository implements ai.EmbeddingRepository for both SQLite and PostgreSQL.
type EmbeddingRepository struct {
	db     *sql.DB
	dbType string
}

// NewEmbeddingRepository creates a new embedding repository.
func NewEmbeddingRepository(db *sql.DB, dbType string) *EmbeddingRepository {
	return &EmbeddingRepository{
		db:     db,
		dbType: dbType,
	}
}

// Store saves or updates an embedding for an entity.
func (r *EmbeddingRepository) Store(ctx context.Context, embedding *ai.Embedding) error {
	now := time.Now()

	if common.IsPostgres(r.dbType) {
		// PostgreSQL with pgvector
		vectorStr := vectorToPostgresString(embedding.Vector)
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO embeddings (entity_type, entity_id, vector, text, dimensions, created_at, updated_at)
			VALUES ($1, $2, $3::vector, $4, $5, $6, $7)
			ON CONFLICT (entity_type, entity_id) 
			DO UPDATE SET vector = $3::vector, text = $4, dimensions = $5, updated_at = $7
		`, embedding.EntityType, embedding.EntityID, vectorStr, embedding.Text, len(embedding.Vector), now, now)
		return err
	}

	// SQLite with BLOB
	vectorBytes := vectorToBytes(embedding.Vector)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO embeddings (entity_type, entity_id, vector, text, dimensions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (entity_type, entity_id) 
		DO UPDATE SET vector = ?, text = ?, dimensions = ?, updated_at = ?
	`, embedding.EntityType, embedding.EntityID, vectorBytes, embedding.Text, len(embedding.Vector), now.Format(time.RFC3339), now.Format(time.RFC3339),
		vectorBytes, embedding.Text, len(embedding.Vector), now.Format(time.RFC3339))
	return err
}

// StoreBatch saves multiple embeddings in a single transaction.
func (r *EmbeddingRepository) StoreBatch(ctx context.Context, embeddings []*ai.Embedding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, emb := range embeddings {
		if err := r.storeInTx(ctx, tx, emb); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *EmbeddingRepository) storeInTx(ctx context.Context, tx *sql.Tx, embedding *ai.Embedding) error {
	now := time.Now()

	if common.IsPostgres(r.dbType) {
		vectorStr := vectorToPostgresString(embedding.Vector)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO embeddings (entity_type, entity_id, vector, text, dimensions, created_at, updated_at)
			VALUES ($1, $2, $3::vector, $4, $5, $6, $7)
			ON CONFLICT (entity_type, entity_id) 
			DO UPDATE SET vector = $3::vector, text = $4, dimensions = $5, updated_at = $7
		`, embedding.EntityType, embedding.EntityID, vectorStr, embedding.Text, len(embedding.Vector), now, now)
		return err
	}

	vectorBytes := vectorToBytes(embedding.Vector)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO embeddings (entity_type, entity_id, vector, text, dimensions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (entity_type, entity_id) 
		DO UPDATE SET vector = ?, text = ?, dimensions = ?, updated_at = ?
	`, embedding.EntityType, embedding.EntityID, vectorBytes, embedding.Text, len(embedding.Vector), now.Format(time.RFC3339), now.Format(time.RFC3339),
		vectorBytes, embedding.Text, len(embedding.Vector), now.Format(time.RFC3339))
	return err
}

// Delete removes an embedding for an entity.
func (r *EmbeddingRepository) Delete(ctx context.Context, entityType ai.EntityType, entityID int64) error {
	if common.IsPostgres(r.dbType) {
		_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings WHERE entity_type = $1 AND entity_id = $2`, entityType, entityID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	return err
}

// DeleteByType removes all embeddings for a given entity type.
func (r *EmbeddingRepository) DeleteByType(ctx context.Context, entityType ai.EntityType) error {
	if common.IsPostgres(r.dbType) {
		_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings WHERE entity_type = $1`, entityType)
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings WHERE entity_type = ?`, entityType)
	return err
}

// DeleteAll removes all embeddings.
func (r *EmbeddingRepository) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings`)
	return err
}

// Get retrieves an embedding by entity type and ID.
func (r *EmbeddingRepository) Get(ctx context.Context, entityType ai.EntityType, entityID int64) (*ai.Embedding, error) {
	var embedding ai.Embedding
	var vectorData interface{}
	var createdAt, updatedAt string

	if common.IsPostgres(r.dbType) {
		var vectorStr string
		err := r.db.QueryRowContext(ctx, `
			SELECT id, entity_type, entity_id, vector::text, text, dimensions, created_at, updated_at
			FROM embeddings WHERE entity_type = $1 AND entity_id = $2
		`, entityType, entityID).Scan(
			&embedding.ID, &embedding.EntityType, &embedding.EntityID,
			&vectorStr, &embedding.Text, &vectorData, &embedding.CreatedAt, &embedding.UpdatedAt,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		embedding.Vector = postgresStringToVector(vectorStr)
		return &embedding, nil
	}

	// SQLite
	var vectorBytes []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, entity_type, entity_id, vector, text, dimensions, created_at, updated_at
		FROM embeddings WHERE entity_type = ? AND entity_id = ?
	`, entityType, entityID).Scan(
		&embedding.ID, &embedding.EntityType, &embedding.EntityID,
		&vectorBytes, &embedding.Text, &vectorData, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	embedding.Vector = bytesToVector(vectorBytes)
	embedding.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	embedding.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &embedding, nil
}

// Search performs a semantic similarity search.
func (r *EmbeddingRepository) Search(ctx context.Context, req ai.SemanticSearchRequest, queryVector []float32) (*ai.SemanticSearchResponse, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	if common.IsPostgres(r.dbType) {
		return r.searchPostgres(ctx, req, queryVector, limit)
	}
	return r.searchSQLite(ctx, req, queryVector, limit)
}

func (r *EmbeddingRepository) searchPostgres(ctx context.Context, req ai.SemanticSearchRequest, queryVector []float32, limit int) (*ai.SemanticSearchResponse, error) {
	vectorStr := vectorToPostgresString(queryVector)

	// Build query with optional type filter
	query := `
		SELECT entity_type, entity_id, text, 1 - (vector <=> $1::vector) as similarity
		FROM embeddings
	`
	args := []interface{}{vectorStr}
	argNum := 2

	if len(req.Types) > 0 {
		placeholders := make([]string, len(req.Types))
		for i, t := range req.Types {
			placeholders[i] = fmt.Sprintf("$%d", argNum)
			args = append(args, string(t))
			argNum++
		}
		query += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	query += fmt.Sprintf(" ORDER BY vector <=> $1::vector LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, limit, req.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []ai.SemanticSearchResult
	for rows.Next() {
		var result ai.SemanticSearchResult
		if err := rows.Scan(&result.EntityType, &result.EntityID, &result.Text, &result.Score); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, result)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM embeddings`
	if len(req.Types) > 0 {
		placeholders := make([]string, len(req.Types))
		countArgs := make([]interface{}, len(req.Types))
		for i, t := range req.Types {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			countArgs[i] = string(t)
		}
		countQuery += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
		var count int
		r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&count)
		return &ai.SemanticSearchResponse{Results: results, TotalCount: count, Query: req.Query}, nil
	}

	var count int
	r.db.QueryRowContext(ctx, countQuery).Scan(&count)

	return &ai.SemanticSearchResponse{
		Results:    results,
		TotalCount: count,
		Query:      req.Query,
	}, nil
}

func (r *EmbeddingRepository) searchSQLite(ctx context.Context, req ai.SemanticSearchRequest, queryVector []float32, limit int) (*ai.SemanticSearchResponse, error) {
	// Use sqlite-vec's vec0 virtual table for fast KNN search
	// Falls back to brute-force if vec0 table doesn't exist

	// Serialize query vector to sqlite-vec format
	queryBytes, err := sqlite_vec.SerializeFloat32(queryVector)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}

	// Check if vec_embeddings table exists (migration may not have run yet)
	var tableExists int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='vec_embeddings'`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		// Fall back to brute-force search
		return r.searchSQLiteBruteForce(ctx, req, queryVector, limit)
	}

	// Build KNN query with vec0
	// Request more results than needed to account for offset
	kValue := limit + req.Offset
	if kValue < 10 {
		kValue = 10 // Minimum k for reasonable results
	}

	var query string
	var args []interface{}

	if len(req.Types) > 0 {
		// Filter by entity type using vec0's metadata column
		placeholders := make([]string, len(req.Types))
		for i, t := range req.Types {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		args = append(args, queryBytes, kValue)

		query = fmt.Sprintf(`
			SELECT v.embedding_id, v.entity_type, v.distance, e.entity_id, e.text
			FROM vec_embeddings v
			JOIN embeddings e ON e.id = v.embedding_id
			WHERE v.entity_type IN (%s)
			  AND v.contents_embedding MATCH ?
			  AND k = ?
			ORDER BY v.distance
		`, strings.Join(placeholders, ","))
	} else {
		args = []interface{}{queryBytes, kValue}
		query = `
			SELECT v.embedding_id, v.entity_type, v.distance, e.entity_id, e.text
			FROM vec_embeddings v
			JOIN embeddings e ON e.id = v.embedding_id
			WHERE v.contents_embedding MATCH ?
			  AND k = ?
			ORDER BY v.distance
		`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		// If vec0 query fails, fall back to brute-force
		return r.searchSQLiteBruteForce(ctx, req, queryVector, limit)
	}
	defer rows.Close()

	var results []ai.SemanticSearchResult
	resultCount := 0

	for rows.Next() {
		var embeddingID int64
		var entityType ai.EntityType
		var distance float32
		var entityID int64
		var text sql.NullString

		if err := rows.Scan(&embeddingID, &entityType, &distance, &entityID, &text); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}

		resultCount++

		// Skip results before offset
		if resultCount <= req.Offset {
			continue
		}

		// Stop after limit
		if len(results) >= limit {
			continue // Keep counting for total
		}

		// Convert distance to similarity score (cosine distance to similarity)
		// sqlite-vec uses cosine distance (0 = identical, 2 = opposite)
		similarity := 1.0 - (distance / 2.0)

		results = append(results, ai.SemanticSearchResult{
			EntityType: entityType,
			EntityID:   entityID,
			Score:      similarity,
			Text:       text.String,
		})
	}

	// Get total count for pagination
	totalCount := resultCount
	if resultCount >= kValue {
		// We hit the k limit, so there may be more results
		// Get actual count from embeddings table
		countQuery := `SELECT COUNT(*) FROM embeddings`
		countArgs := []interface{}{}
		if len(req.Types) > 0 {
			placeholders := make([]string, len(req.Types))
			for i, t := range req.Types {
				placeholders[i] = "?"
				countArgs = append(countArgs, string(t))
			}
			countQuery += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
		}
		r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	}

	return &ai.SemanticSearchResponse{
		Results:    results,
		TotalCount: totalCount,
		Query:      req.Query,
	}, nil
}

// searchSQLiteBruteForce is the fallback method when vec0 is not available.
// It loads all embeddings and computes similarity in Go.
func (r *EmbeddingRepository) searchSQLiteBruteForce(ctx context.Context, req ai.SemanticSearchRequest, queryVector []float32, limit int) (*ai.SemanticSearchResponse, error) {
	query := `SELECT id, entity_type, entity_id, vector, text FROM embeddings`
	args := []interface{}{}

	if len(req.Types) > 0 {
		placeholders := make([]string, len(req.Types))
		for i, t := range req.Types {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		query += fmt.Sprintf(" WHERE entity_type IN (%s)", strings.Join(placeholders, ","))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	type scoredResult struct {
		result ai.SemanticSearchResult
		score  float32
	}
	var scored []scoredResult

	for rows.Next() {
		var id int64
		var entityType ai.EntityType
		var entityID int64
		var vectorBytes []byte
		var text sql.NullString

		if err := rows.Scan(&id, &entityType, &entityID, &vectorBytes, &text); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}

		vector := bytesToVector(vectorBytes)
		similarity := cosineSimilarity(queryVector, vector)

		scored = append(scored, scoredResult{
			result: ai.SemanticSearchResult{
				EntityType: entityType,
				EntityID:   entityID,
				Score:      similarity,
				Text:       text.String,
			},
			score: similarity,
		})
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Apply offset and limit
	start := req.Offset
	if start > len(scored) {
		start = len(scored)
	}
	end := start + limit
	if end > len(scored) {
		end = len(scored)
	}

	results := make([]ai.SemanticSearchResult, 0, end-start)
	for i := start; i < end; i++ {
		results = append(results, scored[i].result)
	}

	return &ai.SemanticSearchResponse{
		Results:    results,
		TotalCount: len(scored),
		Query:      req.Query,
	}, nil
}

// Count returns the total number of embeddings.
func (r *EmbeddingRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embeddings`).Scan(&count)
	return count, err
}

// CountByType returns the number of embeddings for a given entity type.
func (r *EmbeddingRepository) CountByType(ctx context.Context, entityType ai.EntityType) (int64, error) {
	var count int64
	if common.IsPostgres(r.dbType) {
		err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embeddings WHERE entity_type = $1`, entityType).Scan(&count)
		return count, err
	}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embeddings WHERE entity_type = ?`, entityType).Scan(&count)
	return count, err
}

// GetDimensions returns the dimension of stored embeddings (0 if empty).
func (r *EmbeddingRepository) GetDimensions(ctx context.Context) (int, error) {
	var dims int
	err := r.db.QueryRowContext(ctx, `SELECT dimensions FROM embeddings LIMIT 1`).Scan(&dims)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return dims, err
}

// Helper functions

func vectorToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToVector(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func vectorToPostgresString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func postgresStringToVector(s string) []float32 {
	// Remove brackets
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

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// SearchText finds embeddings where text contains the query (case-insensitive).
func (r *EmbeddingRepository) SearchText(ctx context.Context, query string, types []ai.EntityType, limit int) ([]ai.SemanticSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	var args []interface{}
	var whereClause string

	// Build type filter
	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i, t := range types {
			if common.IsPostgres(r.dbType) {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
			} else {
				placeholders[i] = "?"
			}
			args = append(args, string(t))
		}
		whereClause = fmt.Sprintf("entity_type IN (%s) AND ", strings.Join(placeholders, ","))
	}

	// Add text search - use LIKE for case-insensitive search
	searchPattern := "%" + query + "%"
	if common.IsPostgres(r.dbType) {
		args = append(args, searchPattern, limit)
		whereClause += fmt.Sprintf("text ILIKE $%d", len(args)-1)
		sql := fmt.Sprintf(`
			SELECT entity_type, entity_id, text
			FROM embeddings
			WHERE %s
			ORDER BY updated_at DESC
			LIMIT $%d
		`, whereClause, len(args))
		return r.execTextSearch(ctx, sql, args)
	}

	// SQLite - use LIKE (case-insensitive by default)
	args = append(args, searchPattern, limit)
	sql := fmt.Sprintf(`
		SELECT entity_type, entity_id, text
		FROM embeddings
		WHERE %s text LIKE ?
		ORDER BY updated_at DESC
		LIMIT ?
	`, whereClause)
	return r.execTextSearch(ctx, sql, args)
}

func (r *EmbeddingRepository) execTextSearch(ctx context.Context, sql string, args []interface{}) ([]ai.SemanticSearchResult, error) {
	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("text search query: %w", err)
	}
	defer rows.Close()

	var results []ai.SemanticSearchResult
	for rows.Next() {
		var result ai.SemanticSearchResult
		var entityType string
		if err := rows.Scan(&entityType, &result.EntityID, &result.Text); err != nil {
			return nil, fmt.Errorf("scan text search result: %w", err)
		}
		result.EntityType = ai.EntityType(entityType)
		result.Score = 1.0 // Text match = full relevance
		results = append(results, result)
	}

	return results, rows.Err()
}

// Verify interface implementation
var _ ai.EmbeddingRepository = (*EmbeddingRepository)(nil)
