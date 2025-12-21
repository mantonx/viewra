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
	// SQLite doesn't have native vector search, so we load all embeddings and compute similarity in Go
	// This is less efficient but works for smaller datasets
	// For larger datasets, consider using sqlite-vss extension

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

// Verify interface implementation
var _ ai.EmbeddingRepository = (*EmbeddingRepository)(nil)
