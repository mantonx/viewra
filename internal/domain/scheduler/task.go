package scheduler

import (
	"context"
	"errors"
	"time"
)

// TaskSource identifies where a task was registered from.
type TaskSource string

const (
	TaskSourceInternal TaskSource = "internal"
	TaskSourcePlugin   TaskSource = "plugin"
)

// Common errors.
var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrCyclicDependency   = errors.New("cyclic dependency detected")
	ErrDependencyFailed   = errors.New("dependency task failed")
	ErrTaskAlreadyExists  = errors.New("task already exists")
	ErrLockNotAcquired    = errors.New("could not acquire lock")
	ErrTaskAlreadyRunning = errors.New("task is already running")
	ErrMaxConcurrency     = errors.New("max concurrent tasks reached")
)

// Task represents a scheduled task definition.
type Task struct {
	ID          string     `json:"id"`          // Unique ID, e.g., "internal:cleanup:scan-jobs"
	Name        string     `json:"name"`        // Human-readable name
	Description string     `json:"description"` // Description of what the task does
	Schedule    string     `json:"schedule"`    // Cron expression (empty = manual/dependency only)
	Enabled     bool       `json:"enabled"`     // Whether the task is enabled
	Source      TaskSource `json:"source"`      // internal or plugin
	SourceID    string     `json:"source_id"`   // Service name or plugin ID

	// DAG Dependencies
	DependsOn []string `json:"depends_on,omitempty"` // Task IDs this task depends on

	// Execution policy
	TimeoutSeconds int    `json:"timeout_seconds"`  // Max execution time (default 300)
	RetryCount     int    `json:"retry_count"`      // Number of retries on failure
	RetryDelaySecs int    `json:"retry_delay_secs"` // Delay between retries
	ConcurrencyKey string `json:"concurrency_key"`  // Key to prevent concurrent runs

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HasSchedule returns true if this task has a cron schedule.
func (t *Task) HasSchedule() bool {
	return t.Schedule != ""
}

// HasDependencies returns true if this task depends on other tasks.
func (t *Task) HasDependencies() bool {
	return len(t.DependsOn) > 0
}

// TaskStatus extends Task with runtime status information.
type TaskStatus struct {
	Task

	// Runtime status
	IsRunning   bool       `json:"is_running"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	LastSuccess *bool      `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`

	// Dependency status
	DependencyStatus []DependencyInfo `json:"dependency_status,omitempty"`
}

// DependencyInfo provides status information about a task dependency.
type DependencyInfo struct {
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
	LastSuccess *bool  `json:"last_success,omitempty"`
	IsRunning   bool   `json:"is_running"`
}

// TaskHandler is the interface for executing a task.
type TaskHandler interface {
	Execute(ctx context.Context) error
}

// TaskHandlerFunc is an adapter to allow using functions as TaskHandler.
type TaskHandlerFunc func(ctx context.Context) error

// Execute implements TaskHandler.
func (f TaskHandlerFunc) Execute(ctx context.Context) error {
	return f(ctx)
}

// ProgressReporter allows tasks to report progress during execution.
type ProgressReporter interface {
	ReportProgress(percent int, message string)
}

// ProgressHandler is the interface for tasks that support progress reporting.
type ProgressHandler interface {
	ExecuteWithProgress(ctx context.Context, reporter ProgressReporter) error
}

// InternalTask is a convenience struct for registering internal tasks.
type InternalTask struct {
	ID             string
	Name           string
	Description    string
	Schedule       string   // Empty for manual/dependency-only tasks
	DependsOn      []string // Task IDs this task depends on
	Handler        func(ctx context.Context) error
	TimeoutSeconds int
	RetryCount     int
	RetryDelaySecs int
	ConcurrencyKey string
}

// ToTask converts InternalTask to Task domain entity.
func (it InternalTask) ToTask(sourceID string) Task {
	timeout := it.TimeoutSeconds
	if timeout == 0 {
		timeout = 300 // 5 minutes default
	}

	return Task{
		ID:             it.ID,
		Name:           it.Name,
		Description:    it.Description,
		Schedule:       it.Schedule,
		Enabled:        true,
		Source:         TaskSourceInternal,
		SourceID:       sourceID,
		DependsOn:      it.DependsOn,
		TimeoutSeconds: timeout,
		RetryCount:     it.RetryCount,
		RetryDelaySecs: it.RetryDelaySecs,
		ConcurrencyKey: it.ConcurrencyKey,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// TaskUpdate represents allowed updates to a task.
type TaskUpdate struct {
	Schedule *string `json:"schedule,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
}

// Apply applies the update to a task and returns true if anything changed.
func (u TaskUpdate) Apply(t *Task) bool {
	changed := false
	if u.Schedule != nil && *u.Schedule != t.Schedule {
		t.Schedule = *u.Schedule
		changed = true
	}
	if u.Enabled != nil && *u.Enabled != t.Enabled {
		t.Enabled = *u.Enabled
		changed = true
	}
	if changed {
		t.UpdatedAt = time.Now()
	}
	return changed
}
