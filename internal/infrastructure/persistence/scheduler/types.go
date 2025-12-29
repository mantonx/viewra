package scheduler

import (
	"database/sql"
	"encoding/json"

	"github.com/mantonx/viewra/internal/domain/scheduler"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	"github.com/sqlc-dev/pqtype"
)

// parseNullRawMessage extracts bytes from pqtype.NullRawMessage.
func parseNullRawMessage(nrm pqtype.NullRawMessage) []byte {
	if !nrm.Valid {
		return nil
	}
	return nrm.RawMessage
}

// --- Helper Functions ---

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func boolToNullInt64(b bool) sql.NullInt64 {
	return sql.NullInt64{Int64: boolToInt64(b), Valid: true}
}

func boolPtrToNullInt64(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: boolToInt64(*b), Valid: true}
}

func nullBoolPtr(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func nullBoolPtrFromBoolPtr(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func nullBoolPtrToInt64(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Valid: false}
	}
	if *b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func int64ToBool(i int64) bool {
	return i != 0
}

func nullInt64ToBool(i sql.NullInt64) bool {
	return i.Valid && i.Int64 != 0
}

func nullInt64ToBoolPtr(i sql.NullInt64) *bool {
	if !i.Valid {
		return nil
	}
	b := i.Int64 != 0
	return &b
}

func nullBoolToBoolPtr(b sql.NullBool) *bool {
	if !b.Valid {
		return nil
	}
	return &b.Bool
}

// --- SQLite Row Conversions ---

func sqliteRowToTask(row sqlc_sqlite.ScheduledTask) *scheduler.Task {
	task := &scheduler.Task{
		ID:             row.ID,
		Name:           row.Name,
		Description:    common.ParseNullString(row.Description),
		Schedule:       common.ParseNullString(row.Schedule),
		Enabled:        int64ToBool(row.Enabled),
		Source:         scheduler.TaskSource(row.Source),
		SourceID:       common.ParseNullString(row.SourceID),
		TimeoutSeconds: int(common.ParseNullInt64(row.TimeoutSeconds)),
		RetryCount:     int(common.ParseNullInt64(row.RetryCount)),
		RetryDelaySecs: int(common.ParseNullInt64(row.RetryDelaySeconds)),
		ConcurrencyKey: common.ParseNullString(row.ConcurrencyKey),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}

	// Parse depends_on JSON array
	if row.DependsOn.Valid && row.DependsOn.String != "" {
		var deps []string
		if err := json.Unmarshal([]byte(row.DependsOn.String), &deps); err == nil {
			task.DependsOn = deps
		}
	}

	return task
}

func sqliteExecRowToExecution(row sqlc_sqlite.SchedulerExecution) *scheduler.Execution {
	exec := &scheduler.Execution{
		ID:               row.ID,
		TaskID:           row.TaskID,
		Status:           scheduler.ExecutionStatus(row.Status),
		ScheduledAt:      common.ParseNullTimePtr(row.ScheduledAt),
		StartedAt:        common.ParseNullTimePtr(row.StartedAt),
		EndedAt:          common.ParseNullTimePtr(row.EndedAt),
		DurationMs:       common.ParseNullInt64(row.DurationMs),
		Success:          nullInt64ToBoolPtr(row.Success),
		Error:            common.ParseNullString(row.Error),
		Logs:             common.ParseNullString(row.Logs),
		Attempt:          int(common.ParseNullInt64(row.Attempt)),
		ParentID:         common.ParseNullString(row.ParentExecutionID),
		TriggeredBy:      scheduler.TriggeredBy(row.TriggeredBy),
		DependencyExecID: common.ParseNullString(row.DependencyExecID),
		Resumable:        nullInt64ToBool(row.Resumable),
		CreatedAt:        row.CreatedAt,
	}
	return exec
}

// --- PostgreSQL Row Conversions ---

func postgresRowToTask(row sqlc_postgres.ScheduledTask) *scheduler.Task {
	task := &scheduler.Task{
		ID:             row.ID,
		Name:           row.Name,
		Description:    common.ParseNullString(row.Description),
		Schedule:       common.ParseNullString(row.Schedule),
		Enabled:        row.Enabled,
		Source:         scheduler.TaskSource(row.Source),
		SourceID:       common.ParseNullString(row.SourceID),
		TimeoutSeconds: int(common.ParseNullInt32(row.TimeoutSeconds)),
		RetryCount:     int(common.ParseNullInt32(row.RetryCount)),
		RetryDelaySecs: int(common.ParseNullInt32(row.RetryDelaySeconds)),
		ConcurrencyKey: common.ParseNullString(row.ConcurrencyKey),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}

	// Parse depends_on JSONB array
	if depBytes := parseNullRawMessage(row.DependsOn); len(depBytes) > 0 {
		var deps []string
		if err := json.Unmarshal(depBytes, &deps); err == nil {
			task.DependsOn = deps
		}
	}

	return task
}

func postgresExecRowToExecution(row sqlc_postgres.SchedulerExecution) *scheduler.Execution {
	exec := &scheduler.Execution{
		ID:               row.ID,
		TaskID:           row.TaskID,
		Status:           scheduler.ExecutionStatus(row.Status),
		ScheduledAt:      common.ParseNullTimePtr(row.ScheduledAt),
		StartedAt:        common.ParseNullTimePtr(row.StartedAt),
		EndedAt:          common.ParseNullTimePtr(row.EndedAt),
		DurationMs:       int64(common.ParseNullInt32(row.DurationMs)),
		Success:          nullBoolToBoolPtr(row.Success),
		Error:            common.ParseNullString(row.Error),
		Logs:             common.ParseNullString(row.Logs),
		Attempt:          int(common.ParseNullInt32(row.Attempt)),
		ParentID:         common.ParseNullString(row.ParentExecutionID),
		TriggeredBy:      scheduler.TriggeredBy(row.TriggeredBy),
		DependencyExecID: common.ParseNullString(row.DependencyExecID),
		Resumable:        row.Resumable.Valid && row.Resumable.Bool,
		CreatedAt:        row.CreatedAt,
	}
	return exec
}
