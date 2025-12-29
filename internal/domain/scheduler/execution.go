package scheduler

import (
	"time"
)

// ExecutionStatus represents the status of a task execution.
type ExecutionStatus string

const (
	ExecutionStatusPending     ExecutionStatus = "pending"
	ExecutionStatusRunning     ExecutionStatus = "running"
	ExecutionStatusCompleted   ExecutionStatus = "completed"
	ExecutionStatusFailed      ExecutionStatus = "failed"
	ExecutionStatusCancelled   ExecutionStatus = "cancelled"
	ExecutionStatusSkipped     ExecutionStatus = "skipped"     // Dependency failed
	ExecutionStatusInterrupted ExecutionStatus = "interrupted" // Server shutdown/crash
)

// TriggeredBy indicates how an execution was triggered.
type TriggeredBy string

const (
	TriggeredBySchedule   TriggeredBy = "schedule"
	TriggeredByManual     TriggeredBy = "manual"
	TriggeredByRetry      TriggeredBy = "retry"
	TriggeredByDependency TriggeredBy = "dependency" // Parent task completed
)

// MaxLogSize is the maximum size of logs stored per execution (64KB).
const MaxLogSize = 64 * 1024

// Execution represents a single task run.
type Execution struct {
	ID     string          `json:"id"`
	TaskID string          `json:"task_id"`
	Status ExecutionStatus `json:"status"`

	// Timing
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	DurationMs  int64      `json:"duration_ms,omitempty"`

	// Results
	Success *bool  `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
	Logs    string `json:"logs,omitempty"` // Max 64KB, truncated

	// Retry tracking
	Attempt  int    `json:"attempt"`
	ParentID string `json:"parent_execution_id,omitempty"`

	// Context
	TriggeredBy      TriggeredBy `json:"triggered_by"`
	DependencyExecID string      `json:"dependency_exec_id,omitempty"` // Which execution triggered this

	// Resume support
	Resumable bool `json:"resumable"` // Can be resumed after interrupt

	// Progress tracking (transient, not persisted)
	Progress        int    `json:"-"`
	ProgressMessage string `json:"-"`

	CreatedAt time.Time `json:"created_at"`
}

// NewExecution creates a new execution record.
func NewExecution(id, taskID string, triggeredBy TriggeredBy) *Execution {
	now := time.Now()
	return &Execution{
		ID:          id,
		TaskID:      taskID,
		Status:      ExecutionStatusPending,
		Attempt:     1,
		TriggeredBy: triggeredBy,
		CreatedAt:   now,
	}
}

// NewDependentExecution creates a new execution triggered by a dependency completing.
func NewDependentExecution(id, taskID, parentExecID string) *Execution {
	exec := NewExecution(id, taskID, TriggeredByDependency)
	exec.DependencyExecID = parentExecID
	return exec
}

// NewRetryExecution creates a new execution as a retry of a failed one.
func NewRetryExecution(id string, original *Execution) *Execution {
	exec := NewExecution(id, original.TaskID, TriggeredByRetry)
	exec.ParentID = original.ID
	exec.Attempt = original.Attempt + 1
	return exec
}

// Start marks the execution as running.
func (e *Execution) Start() {
	now := time.Now()
	e.Status = ExecutionStatusRunning
	e.StartedAt = &now
}

// Complete marks the execution as completed successfully.
func (e *Execution) Complete(logs string) {
	now := time.Now()
	e.Status = ExecutionStatusCompleted
	e.EndedAt = &now
	success := true
	e.Success = &success
	e.setLogs(logs)
	if e.StartedAt != nil {
		e.DurationMs = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// Fail marks the execution as failed.
func (e *Execution) Fail(err error, logs string) {
	now := time.Now()
	e.Status = ExecutionStatusFailed
	e.EndedAt = &now
	success := false
	e.Success = &success
	e.setLogs(logs)
	if err != nil {
		e.Error = err.Error()
	}
	if e.StartedAt != nil {
		e.DurationMs = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// Cancel marks the execution as cancelled.
func (e *Execution) Cancel() {
	now := time.Now()
	e.Status = ExecutionStatusCancelled
	e.EndedAt = &now
	success := false
	e.Success = &success
	if e.StartedAt != nil {
		e.DurationMs = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// Skip marks the execution as skipped (dependency failed).
func (e *Execution) Skip(reason string) {
	now := time.Now()
	e.Status = ExecutionStatusSkipped
	e.EndedAt = &now
	success := false
	e.Success = &success
	e.Error = reason
}

// Interrupt marks the execution as interrupted (server shutdown).
func (e *Execution) Interrupt(resumable bool) {
	now := time.Now()
	e.Status = ExecutionStatusInterrupted
	e.EndedAt = &now
	e.Resumable = resumable
	if e.StartedAt != nil {
		e.DurationMs = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// UpdateProgress updates the progress tracking fields.
func (e *Execution) UpdateProgress(percent int, message string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	e.Progress = percent
	e.ProgressMessage = message
}

// IsTerminal returns true if the execution is in a terminal state.
func (e *Execution) IsTerminal() bool {
	switch e.Status {
	case ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusCancelled, ExecutionStatusSkipped:
		return true
	default:
		return false
	}
}

// IsResumable returns true if the execution can be resumed.
func (e *Execution) IsResumable() bool {
	return e.Status == ExecutionStatusInterrupted && e.Resumable
}

// CanRetry returns true if the execution can be retried.
func (e *Execution) CanRetry(maxRetries int) bool {
	return e.Status == ExecutionStatusFailed && e.Attempt <= maxRetries
}

// setLogs sets the logs, truncating if necessary.
func (e *Execution) setLogs(logs string) {
	if len(logs) > MaxLogSize {
		e.Logs = logs[:MaxLogSize-100] + "\n\n... [truncated, " + string(rune(len(logs)-MaxLogSize+100)) + " bytes omitted]"
	} else {
		e.Logs = logs
	}
}

// ExecutionListOptions contains options for listing executions.
type ExecutionListOptions struct {
	TaskID string
	Status ExecutionStatus
	Limit  int
	Offset int
}

// ExecutionProgress represents a progress update for an execution.
type ExecutionProgress struct {
	ExecutionID string `json:"execution_id"`
	TaskID      string `json:"task_id"`
	Percent     int    `json:"percent"`
	Message     string `json:"message"`
}
