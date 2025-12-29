package scheduler

import (
	"context"
	"time"
)

// TaskRepository defines persistence operations for scheduled tasks.
type TaskRepository interface {
	// Upsert creates a new task or updates if it already exists.
	Upsert(ctx context.Context, task *Task) error

	// Get retrieves a task by ID.
	Get(ctx context.Context, id string) (*Task, error)

	// List retrieves all tasks.
	List(ctx context.Context) ([]*Task, error)

	// ListBySource retrieves tasks by source (internal or plugin).
	ListBySource(ctx context.Context, source TaskSource) ([]*Task, error)

	// ListBySourceID retrieves tasks by source ID (plugin ID or service name).
	ListBySourceID(ctx context.Context, sourceID string) ([]*Task, error)

	// Update updates a task.
	Update(ctx context.Context, id string, update TaskUpdate) error

	// Delete removes a task by ID.
	Delete(ctx context.Context, id string) error

	// DeleteBySourceID removes all tasks for a source ID.
	DeleteBySourceID(ctx context.Context, sourceID string) error

	// GetDependents returns tasks that depend on the given task ID.
	GetDependents(ctx context.Context, taskID string) ([]*Task, error)
}

// ExecutionRepository defines persistence operations for task executions.
type ExecutionRepository interface {
	// Create creates a new execution record.
	Create(ctx context.Context, exec *Execution) error

	// Get retrieves an execution by ID.
	Get(ctx context.Context, id string) (*Execution, error)

	// Update updates an execution record.
	Update(ctx context.Context, exec *Execution) error

	// List retrieves executions with optional filtering.
	List(ctx context.Context, opts ExecutionListOptions) ([]*Execution, error)

	// GetLatest retrieves the most recent execution for a task.
	GetLatest(ctx context.Context, taskID string) (*Execution, error)

	// GetRunning retrieves all currently running executions.
	GetRunning(ctx context.Context) ([]*Execution, error)

	// GetInterrupted retrieves all interrupted executions that are resumable.
	GetInterrupted(ctx context.Context) ([]*Execution, error)

	// CountByTask counts executions for a task.
	CountByTask(ctx context.Context, taskID string) (int, error)

	// DeleteOlderThan removes executions older than the given time.
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)

	// MarkInterrupted marks all running executions as interrupted.
	// Called on startup to clean up stale running executions.
	MarkInterrupted(ctx context.Context, resumable bool) (int64, error)
}

// LockRepository defines persistence operations for concurrency locks.
type LockRepository interface {
	// TryAcquire attempts to acquire a lock.
	// Returns true if the lock was acquired, false if it's already held.
	TryAcquire(ctx context.Context, key, executionID string, ttl time.Duration) (bool, error)

	// Release releases a lock.
	Release(ctx context.Context, key string) error

	// Refresh extends the TTL of an existing lock.
	Refresh(ctx context.Context, key string, ttl time.Duration) error

	// IsHeld checks if a lock is currently held.
	IsHeld(ctx context.Context, key string) (bool, error)

	// CleanExpired removes all expired locks.
	CleanExpired(ctx context.Context) (int64, error)
}
