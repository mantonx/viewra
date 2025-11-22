package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// DBExecutionLogger implements ExecutionLogger using database storage
type DBExecutionLogger struct {
	queries *sqlc_sqlite.Queries
	logger  *slog.Logger
}

// NewDBExecutionLogger creates a new database-backed execution logger
func NewDBExecutionLogger(db *sql.DB, logger *slog.Logger) *DBExecutionLogger {
	return &DBExecutionLogger{
		queries: sqlc_sqlite.New(db),
		logger:  logger,
	}
}

// LogExecution logs a task execution to the database
func (l *DBExecutionLogger) LogExecution(ctx context.Context, result ExecutionResult) error {
	durationMs := result.Duration.Milliseconds()

	var errorText sql.NullString
	if result.Error != "" {
		errorText = sql.NullString{String: result.Error, Valid: true}
	}

	_, err := l.queries.LogTaskExecution(ctx, sqlc_sqlite.LogTaskExecutionParams{
		TaskID:     result.TaskID,
		StartedAt:  result.StartTime,
		EndedAt:    result.EndTime,
		DurationMs: durationMs,
		Success:    result.Success,
		Error:      errorText,
	})

	if err != nil {
		return fmt.Errorf("failed to log task execution: %w", err)
	}

	l.logger.Debug("Task execution logged",
		"task_id", result.TaskID,
		"success", result.Success,
		"duration_ms", durationMs)

	return nil
}

// GetExecutionHistory retrieves execution history for a task
func (l *DBExecutionLogger) GetExecutionHistory(ctx context.Context, taskID string, limit int) ([]ExecutionResult, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	rows, err := l.queries.GetTaskExecutionHistory(ctx, sqlc_sqlite.GetTaskExecutionHistoryParams{
		TaskID: taskID,
		Limit:  int64(limit),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get task execution history: %w", err)
	}

	results := make([]ExecutionResult, 0, len(rows))
	for _, row := range rows {
		result := ExecutionResult{
			TaskID:    row.TaskID,
			StartTime: row.StartedAt,
			EndTime:   row.EndedAt,
			Duration:  time.Duration(row.DurationMs) * time.Millisecond,
			Success:   row.Success,
		}

		if row.Error.Valid {
			result.Error = row.Error.String
		}

		results = append(results, result)
	}

	return results, nil
}
