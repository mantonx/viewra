package scheduler

import (
	"database/sql"
	"encoding/json"

	"github.com/mantonx/viewra/internal/domain/scheduler"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// --- Helper Functions ---

func boolPtrToNullInt64(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Valid: false}
	}
	if *b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
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

// --- Row Conversions ---

func rowToTask(row unified.ScheduledTask) *scheduler.Task {
	task := &scheduler.Task{
		ID:             row.ID,
		Name:           row.Name,
		Description:    common.ParseNullString(row.Description),
		Schedule:       common.ParseNullString(row.Schedule),
		Enabled:        common.Int64ToBool(row.Enabled),
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

func rowToExecution(row unified.SchedulerExecution) *scheduler.Execution {
	return &scheduler.Execution{
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
}
