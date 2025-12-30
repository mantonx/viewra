package host

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// ensureVectorTablePostgres creates the embeddings table for PostgreSQL if it doesn't exist.
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

// ensureVectorTableSQLite creates the embeddings tables for SQLite if they don't exist.
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

// vectorToPostgresString converts a float32 slice to PostgreSQL vector format.
func vectorToPostgresString(v []float32) string {
	strs := make([]string, len(v))
	for i, f := range v {
		strs[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(strs, ",") + "]"
}

// postgresStringToVector converts a PostgreSQL vector string to float32 slice.
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

// vectorToBytes converts a float32 slice to bytes for SQLite storage.
func vectorToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToVector converts bytes back to a float32 slice.
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

// cosineSimilarity calculates the cosine similarity between two vectors.
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
