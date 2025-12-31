// Package scheduler provides persistence for scheduler task management.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/mantonx/viewra/internal/domain/scheduler"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
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
	*common.BaseRepository
}

// NewTaskRepository creates a new task repository.
func NewTaskRepository(base *common.BaseRepository) *TaskRepository {
	return &TaskRepository{
		BaseRepository: base,
	}
}

// Upsert creates a new task or updates if it already exists.
func (r *TaskRepository) Upsert(ctx context.Context, task *scheduler.Task) error {
	dependsOnJSON, err := json.Marshal(task.DependsOn)
	if err != nil {
		return err
	}

	return r.Q().UpsertScheduledTask(ctx, unified.UpsertScheduledTaskParams{
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

// Get retrieves a task by ID.
func (r *TaskRepository) Get(ctx context.Context, id string) (*scheduler.Task, error) {
	row, err := r.Q().GetScheduledTask(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrTaskNotFound
		}
		return nil, err
	}
	return rowToTask(row), nil
}

// List retrieves all tasks.
func (r *TaskRepository) List(ctx context.Context) ([]*scheduler.Task, error) {
	rows, err := r.Q().ListScheduledTasks(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = rowToTask(row)
	}
	return result, nil
}

// ListBySource retrieves tasks by source (internal or plugin).
func (r *TaskRepository) ListBySource(ctx context.Context, source scheduler.TaskSource) ([]*scheduler.Task, error) {
	rows, err := r.Q().ListScheduledTasksBySource(ctx, string(source))
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = rowToTask(row)
	}
	return result, nil
}

// ListBySourceID retrieves tasks by source ID (plugin ID or service name).
func (r *TaskRepository) ListBySourceID(ctx context.Context, sourceID string) ([]*scheduler.Task, error) {
	rows, err := r.Q().ListScheduledTasksBySourceID(ctx, common.NullString(sourceID))
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Task, len(rows))
	for i, row := range rows {
		result[i] = rowToTask(row)
	}
	return result, nil
}

// Update updates a task.
func (r *TaskRepository) Update(ctx context.Context, id string, update scheduler.TaskUpdate) error {
	return r.Q().UpdateScheduledTask(ctx, unified.UpdateScheduledTaskParams{
		ID:       id,
		Schedule: common.NullStringPtr(update.Schedule),
		Enabled:  nullBoolPtrToInt64(update.Enabled),
	})
}

// Delete removes a task by ID.
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	return r.Q().DeleteScheduledTask(ctx, id)
}

// DeleteBySourceID removes all tasks for a source ID.
func (r *TaskRepository) DeleteBySourceID(ctx context.Context, sourceID string) error {
	return r.Q().DeleteScheduledTasksBySourceID(ctx, common.NullString(sourceID))
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
	*common.BaseRepository
}

// NewExecutionRepository creates a new execution repository.
func NewExecutionRepository(base *common.BaseRepository) *ExecutionRepository {
	return &ExecutionRepository{
		BaseRepository: base,
	}
}

// Create creates a new execution record.
func (r *ExecutionRepository) Create(ctx context.Context, exec *scheduler.Execution) error {
	return r.Q().CreateSchedulerExecution(ctx, unified.CreateSchedulerExecutionParams{
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
	row, err := r.Q().GetSchedulerExecution(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrExecutionNotFound
		}
		return nil, err
	}
	return rowToExecution(row), nil
}

// Update updates an execution record.
func (r *ExecutionRepository) Update(ctx context.Context, exec *scheduler.Execution) error {
	return r.Q().UpdateSchedulerExecution(ctx, unified.UpdateSchedulerExecutionParams{
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

	rows, err := r.Q().ListSchedulerExecutions(ctx, unified.ListSchedulerExecutionsParams{
		TaskID: common.NullString(opts.TaskID),
		Status: common.NullString(string(opts.Status)),
		Limit:  int64(limit),
		Offset: int64(opts.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = rowToExecution(row)
	}
	return result, nil
}

// GetLatest retrieves the most recent execution for a task.
func (r *ExecutionRepository) GetLatest(ctx context.Context, taskID string) (*scheduler.Execution, error) {
	row, err := r.Q().GetLatestSchedulerExecution(ctx, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scheduler.ErrExecutionNotFound
		}
		return nil, err
	}
	return rowToExecution(row), nil
}

// GetRunning retrieves all currently running executions.
func (r *ExecutionRepository) GetRunning(ctx context.Context) ([]*scheduler.Execution, error) {
	rows, err := r.Q().GetRunningSchedulerExecutions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = rowToExecution(row)
	}
	return result, nil
}

// GetInterrupted retrieves all interrupted executions that are resumable.
func (r *ExecutionRepository) GetInterrupted(ctx context.Context) ([]*scheduler.Execution, error) {
	rows, err := r.Q().GetInterruptedSchedulerExecutions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduler.Execution, len(rows))
	for i, row := range rows {
		result[i] = rowToExecution(row)
	}
	return result, nil
}

// CountByTask counts executions for a task.
func (r *ExecutionRepository) CountByTask(ctx context.Context, taskID string) (int, error) {
	count, err := r.Q().CountSchedulerExecutionsByTask(ctx, taskID)
	return int(count), err
}

// DeleteOlderThan removes executions older than the given time.
func (r *ExecutionRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return r.Q().DeleteOldSchedulerExecutions(ctx, before)
}

// MarkInterrupted marks all running executions as interrupted.
func (r *ExecutionRepository) MarkInterrupted(ctx context.Context, resumable bool) (int64, error) {
	return r.Q().MarkRunningAsInterrupted(ctx, common.NullInt64FromBool(resumable))
}

// LockRepository implements scheduler.LockRepository.
type LockRepository struct {
	*common.BaseRepository
}

// NewLockRepository creates a new lock repository.
func NewLockRepository(base *common.BaseRepository) *LockRepository {
	return &LockRepository{
		BaseRepository: base,
	}
}

// TryAcquire attempts to acquire a lock.
func (r *LockRepository) TryAcquire(ctx context.Context, key, executionID string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	err := r.Q().TryAcquireSchedulerLock(ctx, unified.TryAcquireSchedulerLockParams{
		LockKey:     key,
		ExecutionID: executionID,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return false, err
	}

	// Check if we own the lock (ON CONFLICT DO NOTHING means no rows affected if already exists)
	lock, err := r.getLock(ctx, key)
	if err != nil {
		return false, err
	}
	return lock.ExecutionID == executionID, nil
}

func (r *LockRepository) getLock(ctx context.Context, key string) (*lockInfo, error) {
	row, err := r.Q().GetSchedulerLock(ctx, key)
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
	return r.Q().ReleaseSchedulerLock(ctx, key)
}

// Refresh extends the TTL of an existing lock.
func (r *LockRepository) Refresh(ctx context.Context, key string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	return r.Q().RefreshSchedulerLock(ctx, unified.RefreshSchedulerLockParams{
		LockKey:   key,
		ExpiresAt: expiresAt,
	})
}

// IsHeld checks if a lock is currently held.
func (r *LockRepository) IsHeld(ctx context.Context, key string) (bool, error) {
	exists, err := r.Q().SchedulerLockExists(ctx, key)
	return exists > 0, err
}

// CleanExpired removes all expired locks.
func (r *LockRepository) CleanExpired(ctx context.Context) (int64, error) {
	return r.Q().CleanExpiredSchedulerLocks(ctx)
}
