package scheduler

import (
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

// Scheduler event types.
const (
	EventTaskRegistered   events.EventType = "task.registered"
	EventTaskUnregistered events.EventType = "task.unregistered"
	EventTaskUpdated      events.EventType = "task.updated"
	EventTaskEnabled      events.EventType = "task.enabled"
	EventTaskDisabled     events.EventType = "task.disabled"

	EventExecutionScheduled events.EventType = "task.execution.scheduled"
	EventExecutionStarted   events.EventType = "task.execution.started"
	EventExecutionProgress  events.EventType = "task.execution.progress"
	EventExecutionCompleted events.EventType = "task.execution.completed"
	EventExecutionFailed    events.EventType = "task.execution.failed"
	EventExecutionCancelled events.EventType = "task.execution.cancelled"
	EventExecutionRetrying  events.EventType = "task.execution.retrying"
)

// Event data keys.
const (
	DataKeyTaskID      = "task_id"
	DataKeyTaskName    = "task_name"
	DataKeyExecutionID = "execution_id"
	DataKeySchedule    = "schedule"
	DataKeyError       = "error"
	DataKeyAttempt     = "attempt"
	DataKeyDurationMs  = "duration_ms"
	DataKeyProgress    = "progress"
	DataKeySource      = "source"
	DataKeySourceID    = "source_id"
)

// --- Event Structs ---

// ExecutionStartedEvent is published when a task execution begins.
type ExecutionStartedEvent struct {
	ExecutionID string      `json:"execution_id"`
	TaskID      string      `json:"task_id"`
	TriggeredBy TriggeredBy `json:"triggered_by"`
	StartedAt   time.Time   `json:"started_at"`
}

// Type implements events.Event.
func (e ExecutionStartedEvent) Type() events.EventType {
	return EventExecutionStarted
}

// ExecutionCompletedEvent is published when a task execution finishes.
type ExecutionCompletedEvent struct {
	ExecutionID string          `json:"execution_id"`
	TaskID      string          `json:"task_id"`
	Status      ExecutionStatus `json:"status"`
	Success     bool            `json:"success"`
	DurationMs  int64           `json:"duration_ms"`
	Error       string          `json:"error,omitempty"`
}

// Type implements events.Event.
func (e ExecutionCompletedEvent) Type() events.EventType {
	return EventExecutionCompleted
}

// ExecutionProgressEvent is published during task execution to report progress.
type ExecutionProgressEvent struct {
	ExecutionID string `json:"execution_id"`
	TaskID      string `json:"task_id"`
	Percent     int    `json:"percent"`
	Message     string `json:"message,omitempty"`
}

// Type implements events.Event.
func (e ExecutionProgressEvent) Type() events.EventType {
	return EventExecutionProgress
}

// TaskUpdatedEvent is published when a task is updated.
type TaskUpdatedEvent struct {
	TaskID   string `json:"task_id"`
	Schedule string `json:"schedule,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// Type implements events.Event.
func (e TaskUpdatedEvent) Type() events.EventType {
	return EventTaskUpdated
}
