package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Task represents a scheduled task
type Task struct {
	ID          string                        // Unique identifier
	Name        string                        // Human-readable name
	Description string                        // What this task does
	Schedule    string                        // Cron expression (e.g., "0 3 * * *" for 3 AM daily)
	Handler     func(ctx context.Context) error // Task logic
	Enabled     bool                          // Can be disabled
}

// TaskStatus represents the current state of a task
type TaskStatus struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Schedule    string     `json:"schedule"`
	Enabled     bool       `json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	IsRunning   bool       `json:"is_running"`
}

// ExecutionResult represents the outcome of a task execution
type ExecutionResult struct {
	TaskID    string        `json:"task_id"`
	StartTime time.Time     `json:"started_at"`
	EndTime   time.Time     `json:"ended_at"`
	Duration  time.Duration `json:"-"` // Hidden from JSON
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// MarshalJSON customizes JSON serialization for ExecutionResult
func (e ExecutionResult) MarshalJSON() ([]byte, error) {
	type Alias ExecutionResult
	return json.Marshal(&struct {
		DurationMs int64 `json:"duration_ms"`
		*Alias
	}{
		DurationMs: e.Duration.Milliseconds(),
		Alias:      (*Alias)(&e),
	})
}

// ExecutionLogger interface for persisting task execution history
type ExecutionLogger interface {
	LogExecution(ctx context.Context, result ExecutionResult) error
	GetExecutionHistory(ctx context.Context, taskID string, limit int) ([]ExecutionResult, error)
}

// Scheduler manages periodic tasks
type Scheduler struct {
	cron       *cron.Cron
	tasks      map[string]*taskEntry
	mu         sync.RWMutex
	logger     *slog.Logger
	execLogger ExecutionLogger
}

// taskEntry wraps a Task with runtime state
type taskEntry struct {
	task      Task
	cronID    cron.EntryID
	lastRun   *time.Time
	lastError string
	isRunning bool
	runningMu sync.Mutex
}

// Config holds scheduler configuration
type Config struct {
	// Location for cron scheduling (e.g., "America/New_York", "UTC")
	Location string

	// Enable/disable all scheduled tasks
	Enabled bool
}

// DefaultConfig returns default scheduler configuration
func DefaultConfig() Config {
	return Config{
		Location: "Local", // Use server's local timezone
		Enabled:  true,
	}
}

// New creates a new scheduler instance
func New(config Config, logger *slog.Logger, execLogger ExecutionLogger) (*Scheduler, error) {
	// Parse location
	var loc *time.Location
	var err error

	if config.Location == "Local" {
		loc = time.Local
	} else {
		loc, err = time.LoadLocation(config.Location)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
	}

	// Create cron scheduler with options
	c := cron.New(
		cron.WithLocation(loc),
		// Skip cron's built-in logging, we'll log in our handlers
	)

	return &Scheduler{
		cron:       c,
		tasks:      make(map[string]*taskEntry),
		logger:     logger,
		execLogger: execLogger,
	}, nil
}

// RegisterTask adds a new task to the scheduler
func (s *Scheduler) RegisterTask(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate task
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if task.Handler == nil {
		return fmt.Errorf("task handler is required")
	}
	if task.Schedule == "" {
		return fmt.Errorf("task schedule is required")
	}

	// Check if task already exists
	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Parse schedule to validate
	_, err := cron.ParseStandard(task.Schedule)
	if err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	// Create task entry
	entry := &taskEntry{
		task: task,
	}

	// Store task
	s.tasks[task.ID] = entry

	s.logger.Info("Task registered",
		"task_id", task.ID,
		"name", task.Name,
		"schedule", task.Schedule,
		"enabled", task.Enabled)

	return nil
}

// Start begins the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()

	// Schedule all enabled tasks
	for id, entry := range s.tasks {
		if !entry.task.Enabled {
			s.logger.Info("Task disabled, skipping",
				"task_id", id,
				"name", entry.task.Name)
			continue
		}

		// Wrap handler with error handling and logging
		handler := s.wrapHandler(entry)

		// Add to cron
		cronID, err := s.cron.AddFunc(entry.task.Schedule, handler)
		if err != nil {
			s.logger.Error("Failed to schedule task",
				"task_id", id,
				"error", err)
			continue
		}

		entry.cronID = cronID

		// Get next run time
		cronEntry := s.cron.Entry(cronID)
		nextRun := cronEntry.Next

		s.logger.Info("Task scheduled",
			"task_id", id,
			"name", entry.task.Name,
			"next_run", nextRun.Format(time.RFC3339))
	}

	// Start cron scheduler
	s.cron.Start()
	s.logger.Info("Scheduler started", "task_count", len(s.tasks))

	// Unlock before waiting for context cancellation
	s.mu.Unlock()

	// Wait for context cancellation
	<-ctx.Done()
	s.logger.Info("Scheduler stopping...")
	s.Stop()

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.logger.Info("Scheduler stopped")
}

// wrapHandler wraps a task handler for scheduled execution
func (s *Scheduler) wrapHandler(entry *taskEntry) func() {
	return func() {
		s.executeTask(entry)
	}
}

// GetTaskStatus retrieves task status by ID
func (s *Scheduler) GetTaskStatus(id string) (*TaskStatus, error) {
	s.mu.RLock()
	entry, exists := s.tasks[id]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}

	return s.buildTaskStatus(entry), nil
}

// ListTasks returns status for all registered tasks
func (s *Scheduler) ListTasks() []TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]TaskStatus, 0, len(s.tasks))
	for _, entry := range s.tasks {
		statuses = append(statuses, *s.buildTaskStatus(entry))
	}
	return statuses
}

// buildTaskStatus builds a TaskStatus from a taskEntry
func (s *Scheduler) buildTaskStatus(entry *taskEntry) *TaskStatus {
	entry.runningMu.Lock()
	defer entry.runningMu.Unlock()

	status := &TaskStatus{
		ID:          entry.task.ID,
		Name:        entry.task.Name,
		Description: entry.task.Description,
		Schedule:    entry.task.Schedule,
		Enabled:     entry.task.Enabled,
		LastRun:     entry.lastRun,
		LastError:   entry.lastError,
		IsRunning:   entry.isRunning,
	}

	// Get next run time from cron
	if entry.task.Enabled && entry.cronID != 0 {
		for _, cronEntry := range s.cron.Entries() {
			if cronEntry.ID == entry.cronID {
				next := cronEntry.Next
				status.NextRun = &next
				break
			}
		}
	}

	return status
}

// EnableTask enables a task by ID
func (s *Scheduler) EnableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	entry.task.Enabled = true
	s.logger.Info("Task enabled", "task_id", id, "name", entry.task.Name)
	return nil
}

// DisableTask disables a task by ID
func (s *Scheduler) DisableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	entry.task.Enabled = false
	s.logger.Info("Task disabled", "task_id", id, "name", entry.task.Name)
	return nil
}

// UpdateTaskSchedule updates the cron schedule for a task
func (s *Scheduler) UpdateTaskSchedule(id, newSchedule string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	// Validate the new cron schedule
	_, err := s.cron.AddFunc(newSchedule, func() {})
	if err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	// Remove the old cron entry
	s.cron.Remove(entry.cronID)

	// Update the schedule
	entry.task.Schedule = newSchedule

	// Re-register the task with the new schedule
	cronID, err := s.cron.AddFunc(newSchedule, s.wrapHandler(entry))
	if err != nil {
		return fmt.Errorf("failed to register new schedule: %w", err)
	}

	entry.cronID = cronID
	s.logger.Info("Task schedule updated",
		"task_id", id,
		"name", entry.task.Name,
		"new_schedule", newSchedule)

	return nil
}

// TriggerNow executes a task immediately (outside its schedule)
func (s *Scheduler) TriggerNow(taskID string) error {
	s.mu.RLock()
	entry, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if !entry.task.Enabled {
		return fmt.Errorf("task %s is disabled", taskID)
	}

	s.logger.Info("Manual task trigger requested",
		"task_id", taskID,
		"task_name", entry.task.Name)

	// Execute in goroutine to not block caller
	// executeTask handles concurrent execution prevention
	go s.executeTask(entry)

	return nil
}

// executeTask executes a task synchronously (used by manual triggers)
func (s *Scheduler) executeTask(entry *taskEntry) {
	// Mark as running
	entry.runningMu.Lock()
	entry.isRunning = true
	entry.runningMu.Unlock()

	defer func() {
		entry.runningMu.Lock()
		entry.isRunning = false
		entry.runningMu.Unlock()
	}()

	ctx := context.Background()
	startTime := time.Now()

	s.logger.Info("Task execution started",
		"task_id", entry.task.ID,
		"name", entry.task.Name)

	// Execute handler
	err := entry.task.Handler(ctx)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Update task state
	entry.runningMu.Lock()
	entry.lastRun = &endTime
	if err != nil {
		entry.lastError = err.Error()
	} else {
		entry.lastError = ""
	}
	entry.runningMu.Unlock()

	// Log result
	success := err == nil
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	result := ExecutionResult{
		TaskID:    entry.task.ID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Success:   success,
		Error:     errorMsg,
	}

	// Persist to database
	if s.execLogger != nil {
		if logErr := s.execLogger.LogExecution(ctx, result); logErr != nil {
			s.logger.Error("Failed to log task execution",
				"task_id", entry.task.ID,
				"error", logErr)
		}
	}

	if success {
		s.logger.Info("Task execution completed",
			"task_id", entry.task.ID,
			"name", entry.task.Name,
			"duration", duration)
	} else {
		s.logger.Error("Task execution failed",
			"task_id", entry.task.ID,
			"name", entry.task.Name,
			"duration", duration,
			"error", err)
	}
}

// GetExecutionHistory returns execution history for a task
func (s *Scheduler) GetExecutionHistory(ctx context.Context, taskID string, limit int) ([]ExecutionResult, error) {
	if s.execLogger == nil {
		return []ExecutionResult{}, nil
	}
	return s.execLogger.GetExecutionHistory(ctx, taskID, limit)
}
