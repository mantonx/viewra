// Package scheduler provides persistence for scheduler task management.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/mantonx/viewra/internal/domain/scheduler"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// toNullString converts JSON bytes to sql.NullString.
func toNullString(data []byte) sql.NullString {
	if len(data) == 0 || string(data) == "null" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(data), Valid: true}
}

// TaskRepository implements scheduler.TaskRepository.
type TaskRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewTaskRepository creates a new task repository.
func NewTaskRepository(db *sql.DB, dbType string) *TaskRepository {
	r := &TaskRepository{
		db:     db,
		dbType: dbType,
	}
	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}
	return r
}

// Upsert creates a new task or updates if it already exists.
func (r *TaskRepository) Upsert(ctx context.Context, task *scheduler.Task) error {
	dependsOnJSON, err := json.Marshal(task.DependsOn)
	if err != nil {
		return err
	}

	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpsertScheduledTask(ctx, sqlc_postgres.UpsertScheduledTaskParams{
			ID:                task.ID,
			Name:              task.Name,
			Description:       common.NullString(task.Description),
			Schedule:          common.NullString(task.Schedule),
			Enabled:           common.BoolToInt64(task.Enabled),
			Source:            string(task.Source),
			SourceID:          common.NullString(task.SourceID),
			DependsOn:         toNullString(dependsOnJSON),
			TimeoutSeconds:    common.NullInt64(int64(task.TimeoutSeconds)),
			RetryCount:        common.NullInt64(int64(task.RetryCount)),
			RetryDelaySeconds: common.NullInt64(int64(task.RetryDelaySecs)),
			ConcurrencyKey:    common.NullString(task.ConcurrencyKey),
		})
	}

	return r.sqliteQuerier.UpsertScheduledTask(ctx, sqlc_sqlite.UpsertScheduledTaskParams{
		ID:                task.ID,
		Name:              task.Name,
		Description:       common.NullString(task.Description),
		Schedule:          common.NullString(task.Schedule),
		Enabled:           boolToInt64(task.Enabled),
		Source:            string(task.Source),
		SourceID:          common.NullString(task.SourceID),
		DependsOn:         common.NullString(string(dependsOnJSON)),
		TimeoutSeconds:    common.NullInt64(int64(task.TimeoutSeconds)),
		RetryCount:        common.NullInt64(int64(task.RetryCount)),
		RetryDelaySeconds: common.NullInt64(int64(task.RetryDelaySecs)),
		ConcurrencyKey:    common.NullString(task.ConcurrencyKey),
	})
}

// Get retrieves a task by ID.
func (r *TaskRepository) Get(ctx context.Context, id string) (*scheduler.Task, error) {
	if common.IsPostgres(r.dbType) {
		row, err := r.postgresQuerier.GetScheduledTask(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, scheduler.ErrTaskNotFound
			}
			return nil, err
		}
		return postgresRowToTask(row), nil
	}

	row, err := r.sqliteQuerier.GetScheduledTask(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrTaskNotFound
		}
		return nil, err
	}
	return sqliteRowToTask(row), nil
}

// List retrieves all tasks.
func (r *TaskRepository) List(ctx context.Context) ([]*scheduler.Task, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.ListScheduledTasks(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Task, len(rows))
		for i, row := range rows {
			result[i] = postgresRowToTask(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.ListScheduledTasks(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = sqliteRowToTask(row)
	}
	return result, nil
}

// ListBySource retrieves tasks by source (internal or plugin).
func (r *TaskRepository) ListBySource(ctx context.Context, source scheduler.TaskSource) ([]*scheduler.Task, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.ListScheduledTasksBySource(ctx, string(source))
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Task, len(rows))
		for i, row := range rows {
			result[i] = postgresRowToTask(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.ListScheduledTasksBySource(ctx, string(source))
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = sqliteRowToTask(row)
	}
	return result, nil
}

// ListBySourceID retrieves tasks by source ID (plugin ID or service name).
func (r *TaskRepository) ListBySourceID(ctx context.Context, sourceID string) ([]*scheduler.Task, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.ListScheduledTasksBySourceID(ctx, common.NullString(sourceID))
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Task, len(rows))
		for i, row := range rows {
			result[i] = postgresRowToTask(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.ListScheduledTasksBySourceID(ctx, common.NullString(sourceID))
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = sqliteRowToTask(row)
	}
	return result, nil
}

// Update updates a task.
func (r *TaskRepository) Update(ctx context.Context, id string, update scheduler.TaskUpdate) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpdateScheduledTask(ctx, sqlc_postgres.UpdateScheduledTaskParams{
			ID:       id,
			Schedule: common.NullStringPtr(update.Schedule),
			Enabled:  nullBoolPtrToInt64(update.Enabled),
		})
	}

	return r.sqliteQuerier.UpdateScheduledTask(ctx, sqlc_sqlite.UpdateScheduledTaskParams{
		ID:       id,
		Schedule: common.NullStringPtr(update.Schedule),
		Enabled:  nullBoolPtrToInt64(update.Enabled),
	})
}

// Delete removes a task by ID.
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.DeleteScheduledTask(ctx, id)
	}
	return r.sqliteQuerier.DeleteScheduledTask(ctx, id)
}

// DeleteBySourceID removes all tasks for a source ID.
func (r *TaskRepository) DeleteBySourceID(ctx context.Context, sourceID string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.DeleteScheduledTasksBySourceID(ctx, common.NullString(sourceID))
	}
	return r.sqliteQuerier.DeleteScheduledTasksBySourceID(ctx, common.NullString(sourceID))
}

// GetDependents returns tasks that depend on the given task ID.
func (r *TaskRepository) GetDependents(ctx context.Context, taskID string) ([]*scheduler.Task, error) {
	// We need to fetch all tasks and filter in memory since JSON queries vary by DB
	tasks, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var dependents []*scheduler.Task
	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			if dep == taskID {
				dependents = append(dependents, task)
				break
			}
		}
	}
	return dependents, nil
}

// ExecutionRepository implements scheduler.ExecutionRepository.
type ExecutionRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewExecutionRepository creates a new execution repository.
func NewExecutionRepository(db *sql.DB, dbType string) *ExecutionRepository {
	r := &ExecutionRepository{
		db:     db,
		dbType: dbType,
	}
	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}
	return r
}

// Create creates a new execution record.
func (r *ExecutionRepository) Create(ctx context.Context, exec *scheduler.Execution) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.CreateSchedulerExecution(ctx, sqlc_postgres.CreateSchedulerExecutionParams{
			ID:                exec.ID,
			TaskID:            exec.TaskID,
			Status:            string(exec.Status),
			ScheduledAt:       common.NullTimePtr(exec.ScheduledAt),
			StartedAt:         common.NullTimePtr(exec.StartedAt),
			EndedAt:           common.NullTimePtr(exec.EndedAt),
			DurationMs:        common.NullInt64(exec.DurationMs),
			Success:           boolPtrToNullInt64(exec.Success),
			Error:             common.NullString(exec.Error),
			Logs:              common.NullString(exec.Logs),
			Attempt:           common.NullInt64(int64(exec.Attempt)),
			ParentExecutionID: common.NullString(exec.ParentID),
			TriggeredBy:       string(exec.TriggeredBy),
			DependencyExecID:  common.NullString(exec.DependencyExecID),
			Resumable:         common.NullInt64FromBool(exec.Resumable),
		})
	}

	return r.sqliteQuerier.CreateSchedulerExecution(ctx, sqlc_sqlite.CreateSchedulerExecutionParams{
		ID:                exec.ID,
		TaskID:            exec.TaskID,
		Status:            string(exec.Status),
		ScheduledAt:       common.NullTimePtr(exec.ScheduledAt),
		StartedAt:         common.NullTimePtr(exec.StartedAt),
		EndedAt:           common.NullTimePtr(exec.EndedAt),
		DurationMs:        common.NullInt64(exec.DurationMs),
		Success:           boolPtrToNullInt64(exec.Success),
		Error:             common.NullString(exec.Error),
		Logs:              common.NullString(exec.Logs),
		Attempt:           common.NullInt64(int64(exec.Attempt)),
		ParentExecutionID: common.NullString(exec.ParentID),
		TriggeredBy:       string(exec.TriggeredBy),
		DependencyExecID:  common.NullString(exec.DependencyExecID),
		Resumable:         common.NullInt64FromBool(exec.Resumable),
	})
}

// Get retrieves an execution by ID.
func (r *ExecutionRepository) Get(ctx context.Context, id string) (*scheduler.Execution, error) {
	if common.IsPostgres(r.dbType) {
		row, err := r.postgresQuerier.GetSchedulerExecution(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, scheduler.ErrExecutionNotFound
			}
			return nil, err
		}
		return postgresExecRowToExecution(row), nil
	}

	row, err := r.sqliteQuerier.GetSchedulerExecution(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrExecutionNotFound
		}
		return nil, err
	}
	return sqliteExecRowToExecution(row), nil
}

// Update updates an execution record.
func (r *ExecutionRepository) Update(ctx context.Context, exec *scheduler.Execution) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpdateSchedulerExecution(ctx, sqlc_postgres.UpdateSchedulerExecutionParams{
			ID:         exec.ID,
			Status:     string(exec.Status),
			StartedAt:  common.NullTimePtr(exec.StartedAt),
			EndedAt:    common.NullTimePtr(exec.EndedAt),
			DurationMs: common.NullInt64(exec.DurationMs),
			Success:    boolPtrToNullInt64(exec.Success),
			Error:      common.NullString(exec.Error),
			Logs:       common.NullString(exec.Logs),
			Resumable:  common.NullInt64FromBool(exec.Resumable),
		})
	}

	return r.sqliteQuerier.UpdateSchedulerExecution(ctx, sqlc_sqlite.UpdateSchedulerExecutionParams{
		ID:         exec.ID,
		Status:     string(exec.Status),
		StartedAt:  common.NullTimePtr(exec.StartedAt),
		EndedAt:    common.NullTimePtr(exec.EndedAt),
		DurationMs: common.NullInt64(exec.DurationMs),
		Success:    boolPtrToNullInt64(exec.Success),
		Error:      common.NullString(exec.Error),
		Logs:       common.NullString(exec.Logs),
		Resumable:  common.NullInt64FromBool(exec.Resumable),
	})
}

// List retrieves executions with optional filtering.
func (r *ExecutionRepository) List(ctx context.Context, opts scheduler.ExecutionListOptions) ([]*scheduler.Execution, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.ListSchedulerExecutions(ctx, sqlc_postgres.ListSchedulerExecutionsParams{
			TaskID: common.NullString(opts.TaskID),
			Status: common.NullString(string(opts.Status)),
			Limit:  int32(limit),
			Offset: int32(opts.Offset),
		})
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Execution, len(rows))
		for i, row := range rows {
			result[i] = postgresExecRowToExecution(row)
		}
		return result, nil
	}

	var taskID, status interface{}
	if opts.TaskID != "" {
		taskID = opts.TaskID
	}
	if opts.Status != "" {
		status = string(opts.Status)
	}
	rows, err := r.sqliteQuerier.ListSchedulerExecutions(ctx, sqlc_sqlite.ListSchedulerExecutionsParams{
		TaskID: taskID,
		Status: status,
		Limit:  int64(limit),
		Offset: int64(opts.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = sqliteExecRowToExecution(row)
	}
	return result, nil
}

// GetLatest retrieves the most recent execution for a task.
func (r *ExecutionRepository) GetLatest(ctx context.Context, taskID string) (*scheduler.Execution, error) {
	if common.IsPostgres(r.dbType) {
		row, err := r.postgresQuerier.GetLatestSchedulerExecution(ctx, taskID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, scheduler.ErrExecutionNotFound
			}
			return nil, err
		}
		return postgresExecRowToExecution(row), nil
	}

	row, err := r.sqliteQuerier.GetLatestSchedulerExecution(ctx, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrExecutionNotFound
		}
		return nil, err
	}
	return sqliteExecRowToExecution(row), nil
}

// GetRunning retrieves all currently running executions.
func (r *ExecutionRepository) GetRunning(ctx context.Context) ([]*scheduler.Execution, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.GetRunningSchedulerExecutions(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Execution, len(rows))
		for i, row := range rows {
			result[i] = postgresExecRowToExecution(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.GetRunningSchedulerExecutions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = sqliteExecRowToExecution(row)
	}
	return result, nil
}

// GetInterrupted retrieves all interrupted executions that are resumable.
func (r *ExecutionRepository) GetInterrupted(ctx context.Context) ([]*scheduler.Execution, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.GetInterruptedSchedulerExecutions(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*scheduler.Execution, len(rows))
		for i, row := range rows {
			result[i] = postgresExecRowToExecution(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.GetInterruptedSchedulerExecutions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = sqliteExecRowToExecution(row)
	}
	return result, nil
}

// CountByTask counts executions for a task.
func (r *ExecutionRepository) CountByTask(ctx context.Context, taskID string) (int, error) {
	if common.IsPostgres(r.dbType) {
		count, err := r.postgresQuerier.CountSchedulerExecutionsByTask(ctx, taskID)
		return int(count), err
	}
	count, err := r.sqliteQuerier.CountSchedulerExecutionsByTask(ctx, taskID)
	return int(count), err
}

// DeleteOlderThan removes executions older than the given time.
func (r *ExecutionRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.DeleteOldSchedulerExecutions(ctx, before)
	}
	return r.sqliteQuerier.DeleteOldSchedulerExecutions(ctx, before)
}

// MarkInterrupted marks all running executions as interrupted.
func (r *ExecutionRepository) MarkInterrupted(ctx context.Context, resumable bool) (int64, error) {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.MarkRunningAsInterrupted(ctx, common.NullInt64FromBool(resumable))
	}
	return r.sqliteQuerier.MarkRunningAsInterrupted(ctx, common.NullInt64FromBool(resumable))
}

// LockRepository implements scheduler.LockRepository.
type LockRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewLockRepository creates a new lock repository.
func NewLockRepository(db *sql.DB, dbType string) *LockRepository {
	r := &LockRepository{
		db:     db,
		dbType: dbType,
	}
	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}
	return r
}

// TryAcquire attempts to acquire a lock.
func (r *LockRepository) TryAcquire(ctx context.Context, key, executionID string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	if common.IsPostgres(r.dbType) {
		err := r.postgresQuerier.TryAcquireSchedulerLock(ctx, sqlc_postgres.TryAcquireSchedulerLockParams{
			LockKey:     key,
			ExecutionID: executionID,
			ExpiresAt:   expiresAt,
		})
		if err != nil {
			return false, err
		}
	} else {
		err := r.sqliteQuerier.TryAcquireSchedulerLock(ctx, sqlc_sqlite.TryAcquireSchedulerLockParams{
			LockKey:     key,
			ExecutionID: executionID,
			ExpiresAt:   expiresAt,
		})
		if err != nil {
			return false, err
		}
	}

	// Check if we own the lock (ON CONFLICT DO NOTHING means no rows affected if already exists)
	lock, err := r.getLock(ctx, key)
	if err != nil {
		return false, err
	}
	return lock.ExecutionID == executionID, nil
}

func (r *LockRepository) getLock(ctx context.Context, key string) (*lockInfo, error) {
	if common.IsPostgres(r.dbType) {
		row, err := r.postgresQuerier.GetSchedulerLock(ctx, key)
		if err != nil {
			return nil, err
		}
		return &lockInfo{
			LockKey:     row.LockKey,
			ExecutionID: row.ExecutionID,
			AcquiredAt:  row.AcquiredAt,
			ExpiresAt:   row.ExpiresAt,
		}, nil
	}

	row, err := r.sqliteQuerier.GetSchedulerLock(ctx, key)
	if err != nil {
		return nil, err
	}
	return &lockInfo{
		LockKey:     row.LockKey,
		ExecutionID: row.ExecutionID,
		AcquiredAt:  row.AcquiredAt,
		ExpiresAt:   row.ExpiresAt,
	}, nil
}

type lockInfo struct {
	LockKey     string
	ExecutionID string
	AcquiredAt  time.Time
	ExpiresAt   time.Time
}

// Release releases a lock.
func (r *LockRepository) Release(ctx context.Context, key string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.ReleaseSchedulerLock(ctx, key)
	}
	return r.sqliteQuerier.ReleaseSchedulerLock(ctx, key)
}

// Refresh extends the TTL of an existing lock.
func (r *LockRepository) Refresh(ctx context.Context, key string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.RefreshSchedulerLock(ctx, sqlc_postgres.RefreshSchedulerLockParams{
			LockKey:   key,
			ExpiresAt: expiresAt,
		})
	}
	return r.sqliteQuerier.RefreshSchedulerLock(ctx, sqlc_sqlite.RefreshSchedulerLockParams{
		LockKey:   key,
		ExpiresAt: expiresAt,
	})
}

// IsHeld checks if a lock is currently held.
func (r *LockRepository) IsHeld(ctx context.Context, key string) (bool, error) {
	if common.IsPostgres(r.dbType) {
		exists, err := r.postgresQuerier.SchedulerLockExists(ctx, key)
		return exists, err
	}
	exists, err := r.sqliteQuerier.SchedulerLockExists(ctx, key)
	return exists == 1, err
}

// CleanExpired removes all expired locks.
func (r *LockRepository) CleanExpired(ctx context.Context) (int64, error) {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.CleanExpiredSchedulerLocks(ctx)
	}
	return r.sqliteQuerier.CleanExpiredSchedulerLocks(ctx)
}
