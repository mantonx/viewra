// Package scheduler provides the application-level scheduler service.
package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/domain/scheduler"
	"github.com/robfig/cron/v3"
)

// Config holds scheduler service configuration.
type Config struct {
	// Location for cron scheduling (e.g., "America/New_York", "UTC", "Local")
	Location string

	// HistoryRetentionDays is how long to keep execution history (default 30)
	HistoryRetentionDays int

	// DefaultTimeout is the default task timeout in seconds (default 300)
	DefaultTimeout int
}

// DefaultConfig returns default scheduler configuration.
func DefaultConfig() Config {
	return Config{
		Location:             "Local",
		HistoryRetentionDays: 30,
		DefaultTimeout:       300,
	}
}

// Service manages scheduled tasks with persistence and DAG support.
type Service struct {
	config   Config
	logger   *slog.Logger
	taskRepo scheduler.TaskRepository
	execRepo scheduler.ExecutionRepository
	lockRepo scheduler.LockRepository
	eventBus EventPublisher

	cron     *cron.Cron
	handlers map[string]scheduler.TaskHandler // task ID -> handler
	cronIDs  map[string]cron.EntryID          // task ID -> cron entry ID

	running   map[string]*runningTask // execution ID -> running task
	runningMu sync.RWMutex

	mu      sync.RWMutex
	started bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// runningTask tracks a currently executing task.
type runningTask struct {
	exec   *scheduler.Execution
	cancel context.CancelFunc
	logBuf *bytes.Buffer
}

// EventPublisher publishes scheduler events.
type EventPublisher interface {
	Publish(ctx context.Context, event interface{})
}

// NewService creates a new scheduler service.
func NewService(
	config Config,
	logger *slog.Logger,
	taskRepo scheduler.TaskRepository,
	execRepo scheduler.ExecutionRepository,
	lockRepo scheduler.LockRepository,
	eventBus EventPublisher,
) (*Service, error) {
	// Parse location
	var loc *time.Location
	var err error

	if config.Location == "Local" || config.Location == "" {
		loc = time.Local
	} else {
		loc, err = time.LoadLocation(config.Location)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", config.Location, err)
		}
	}

	// Set defaults
	if config.HistoryRetentionDays <= 0 {
		config.HistoryRetentionDays = 30
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 300
	}

	return &Service{
		config:   config,
		logger:   logger,
		taskRepo: taskRepo,
		execRepo: execRepo,
		lockRepo: lockRepo,
		eventBus: eventBus,
		cron:     cron.New(cron.WithLocation(loc)),
		handlers: make(map[string]scheduler.TaskHandler),
		cronIDs:  make(map[string]cron.EntryID),
		running:  make(map[string]*runningTask),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}, nil
}

// RegisterHandler registers a handler for a task ID.
// This must be called before Start for internal tasks.
func (s *Service) RegisterHandler(taskID string, handler scheduler.TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[taskID] = handler
}

// RegisterInternalTask registers an internal task with its handler.
// The task definition is persisted to the database.
func (s *Service) RegisterInternalTask(ctx context.Context, task scheduler.InternalTask, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Register handler
	s.handlers[task.ID] = scheduler.TaskHandlerFunc(task.Handler)

	// Persist task definition
	domainTask := task.ToTask(sourceID)
	if err := s.taskRepo.Upsert(ctx, &domainTask); err != nil {
		return fmt.Errorf("failed to persist task %s: %w", task.ID, err)
	}

	s.logger.Info("Registered internal task",
		"task_id", task.ID,
		"name", task.Name,
		"schedule", task.Schedule)

	return nil
}

// Start begins the scheduler service.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already started")
	}
	s.started = true
	s.mu.Unlock()

	// Mark any running executions as interrupted (from previous crash/shutdown)
	if count, err := s.execRepo.MarkInterrupted(ctx, false); err != nil {
		s.logger.Error("Failed to mark interrupted executions", "error", err)
	} else if count > 0 {
		s.logger.Info("Marked stale executions as interrupted", "count", count)
	}

	// Clean expired locks
	if count, err := s.lockRepo.CleanExpired(ctx); err != nil {
		s.logger.Error("Failed to clean expired locks", "error", err)
	} else if count > 0 {
		s.logger.Info("Cleaned expired locks", "count", count)
	}

	// Load all tasks and schedule enabled ones
	tasks, err := s.taskRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	for _, task := range tasks {
		if err := s.scheduleTask(task); err != nil {
			s.logger.Error("Failed to schedule task",
				"task_id", task.ID,
				"error", err)
		}
	}

	// Start cron scheduler
	s.cron.Start()
	s.logger.Info("Scheduler started", "task_count", len(tasks))

	// Start background cleanup goroutine
	go s.cleanupLoop(ctx)

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully stops the scheduler.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	close(s.stopCh)
	s.mu.Unlock()

	// Stop accepting new cron jobs
	cronCtx := s.cron.Stop()

	// Wait for cron to finish current jobs (with timeout)
	select {
	case <-cronCtx.Done():
	case <-time.After(30 * time.Second):
		s.logger.Warn("Cron shutdown timed out")
	}

	// Mark all running executions as interrupted
	s.runningMu.Lock()
	for _, rt := range s.running {
		rt.cancel()
		rt.exec.Interrupt(false) // Not resumable by default
		if err := s.execRepo.Update(context.Background(), rt.exec); err != nil {
			s.logger.Error("Failed to mark execution as interrupted",
				"execution_id", rt.exec.ID,
				"error", err)
		}
	}
	s.running = make(map[string]*runningTask)
	s.runningMu.Unlock()

	close(s.doneCh)
	s.logger.Info("Scheduler stopped")
}

// scheduleTask adds a task to the cron scheduler.
func (s *Service) scheduleTask(task *scheduler.Task) error {
	if !task.Enabled {
		s.logger.Debug("Task disabled, not scheduling", "task_id", task.ID)
		return nil
	}

	if task.Schedule == "" {
		s.logger.Debug("Task has no schedule (manual/dependency only)", "task_id", task.ID)
		return nil
	}

	// Check if we have a handler
	handler, ok := s.handlers[task.ID]
	if !ok {
		s.logger.Warn("No handler registered for task", "task_id", task.ID)
		return nil
	}

	// Create wrapper that executes the task
	taskID := task.ID
	wrapper := func() {
		s.executeScheduledTask(taskID, handler)
	}

	// Add to cron
	cronID, err := s.cron.AddFunc(task.Schedule, wrapper)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", task.Schedule, err)
	}

	s.mu.Lock()
	s.cronIDs[task.ID] = cronID
	s.mu.Unlock()

	// Log next run time
	entry := s.cron.Entry(cronID)
	s.logger.Info("Task scheduled",
		"task_id", task.ID,
		"name", task.Name,
		"schedule", task.Schedule,
		"next_run", entry.Next.Format(time.RFC3339))

	return nil
}

// executeScheduledTask is called by cron for scheduled executions.
func (s *Service) executeScheduledTask(taskID string, handler scheduler.TaskHandler) {
	ctx := context.Background()

	// Load task from DB to get current config
	task, err := s.taskRepo.Get(ctx, taskID)
	if err != nil {
		s.logger.Error("Failed to load task for execution",
			"task_id", taskID,
			"error", err)
		return
	}

	if !task.Enabled {
		s.logger.Debug("Task disabled, skipping scheduled execution", "task_id", taskID)
		return
	}

	// Execute with schedule trigger
	s.execute(ctx, task, handler, scheduler.TriggeredBySchedule, "")
}

// TriggerTask manually triggers a task execution.
func (s *Service) TriggerTask(ctx context.Context, taskID string) (*scheduler.Execution, error) {
	task, err := s.taskRepo.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	handler, ok := s.handlers[taskID]
	if !ok {
		return nil, fmt.Errorf("no handler registered for task %s", taskID)
	}

	exec := s.execute(ctx, task, handler, scheduler.TriggeredByManual, "")
	return exec, nil
}

// execute runs a task and handles all execution lifecycle.
func (s *Service) execute(
	ctx context.Context,
	task *scheduler.Task,
	handler scheduler.TaskHandler,
	triggeredBy scheduler.TriggeredBy,
	dependencyExecID string,
) *scheduler.Execution {
	execID := uuid.New().String()

	// Create execution record
	exec := scheduler.NewExecution(execID, task.ID, triggeredBy)
	exec.DependencyExecID = dependencyExecID

	// Try to acquire lock if task has concurrency key
	lockKey := task.ConcurrencyKey
	if lockKey == "" {
		lockKey = task.ID // Default to task ID
	}

	acquired, err := s.lockRepo.TryAcquire(ctx, lockKey, execID, time.Duration(task.TimeoutSeconds)*time.Second)
	if err != nil {
		s.logger.Error("Failed to acquire lock",
			"task_id", task.ID,
			"lock_key", lockKey,
			"error", err)
		exec.Fail(fmt.Errorf("failed to acquire lock: %w", err), "")
		s.execRepo.Create(ctx, exec)
		return exec
	}

	if !acquired {
		s.logger.Info("Task already running, skipping",
			"task_id", task.ID,
			"lock_key", lockKey)
		exec.Skip("task already running")
		s.execRepo.Create(ctx, exec)
		return exec
	}

	// Persist initial execution record
	if err := s.execRepo.Create(ctx, exec); err != nil {
		s.logger.Error("Failed to create execution record",
			"task_id", task.ID,
			"error", err)
		s.lockRepo.Release(ctx, lockKey)
		return exec
	}

	// Create cancellable context with timeout
	timeout := time.Duration(task.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = time.Duration(s.config.DefaultTimeout) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)

	// Track running task
	logBuf := &bytes.Buffer{}
	rt := &runningTask{
		exec:   exec,
		cancel: cancel,
		logBuf: logBuf,
	}

	s.runningMu.Lock()
	s.running[execID] = rt
	s.runningMu.Unlock()

	// Publish start event
	s.publishEvent(ctx, scheduler.ExecutionStartedEvent{
		ExecutionID: execID,
		TaskID:      task.ID,
		TriggeredBy: triggeredBy,
		StartedAt:   time.Now(),
	})

	// Mark as running
	exec.Start()
	s.execRepo.Update(ctx, exec)

	s.logger.Info("Task execution started",
		"task_id", task.ID,
		"execution_id", execID,
		"triggered_by", triggeredBy)

	// Execute handler
	execErr := handler.Execute(execCtx)

	// Clean up
	cancel()
	s.lockRepo.Release(ctx, lockKey)

	s.runningMu.Lock()
	delete(s.running, execID)
	s.runningMu.Unlock()

	// Update execution result
	logs := logBuf.String()
	if execErr != nil {
		exec.Fail(execErr, logs)
		s.logger.Error("Task execution failed",
			"task_id", task.ID,
			"execution_id", execID,
			"duration_ms", exec.DurationMs,
			"error", execErr)
	} else {
		exec.Complete(logs)
		s.logger.Info("Task execution completed",
			"task_id", task.ID,
			"execution_id", execID,
			"duration_ms", exec.DurationMs)
	}

	s.execRepo.Update(ctx, exec)

	// Publish completion event
	s.publishEvent(ctx, scheduler.ExecutionCompletedEvent{
		ExecutionID: execID,
		TaskID:      task.ID,
		Status:      exec.Status,
		Success:     exec.Success != nil && *exec.Success,
		DurationMs:  exec.DurationMs,
		Error:       exec.Error,
	})

	// Trigger dependent tasks if successful
	if exec.Success != nil && *exec.Success {
		s.triggerDependents(ctx, task.ID, execID)
	}

	return exec
}

// triggerDependents triggers tasks that depend on the completed task.
func (s *Service) triggerDependents(ctx context.Context, taskID, parentExecID string) {
	dependents, err := s.taskRepo.GetDependents(ctx, taskID)
	if err != nil {
		s.logger.Error("Failed to get dependent tasks",
			"task_id", taskID,
			"error", err)
		return
	}

	for _, dep := range dependents {
		if !dep.Enabled {
			continue
		}

		handler, ok := s.handlers[dep.ID]
		if !ok {
			s.logger.Warn("No handler for dependent task",
				"task_id", dep.ID,
				"parent_task_id", taskID)
			continue
		}

		s.logger.Info("Triggering dependent task",
			"task_id", dep.ID,
			"parent_task_id", taskID,
			"parent_exec_id", parentExecID)

		// Execute dependent task asynchronously
		go s.execute(ctx, dep, handler, scheduler.TriggeredByDependency, parentExecID)
	}
}

// cleanupLoop periodically cleans old execution history.
func (s *Service) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	s.cleanupOldExecutions(ctx)

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanupOldExecutions(ctx)
		}
	}
}

// cleanupOldExecutions removes execution history older than retention period.
func (s *Service) cleanupOldExecutions(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -s.config.HistoryRetentionDays)
	count, err := s.execRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		s.logger.Error("Failed to clean old executions", "error", err)
		return
	}
	if count > 0 {
		s.logger.Info("Cleaned old executions",
			"count", count,
			"retention_days", s.config.HistoryRetentionDays)
	}
}

// publishEvent publishes an event if event bus is configured.
func (s *Service) publishEvent(ctx context.Context, event interface{}) {
	if s.eventBus != nil {
		s.eventBus.Publish(ctx, event)
	}
}

// --- Query Methods ---

// GetTask retrieves a task by ID.
func (s *Service) GetTask(ctx context.Context, id string) (*scheduler.Task, error) {
	return s.taskRepo.Get(ctx, id)
}

// ListTasks returns all tasks.
func (s *Service) ListTasks(ctx context.Context) ([]*scheduler.Task, error) {
	return s.taskRepo.List(ctx)
}

// GetTaskStatus returns a task with runtime status information.
func (s *Service) GetTaskStatus(ctx context.Context, id string) (*scheduler.TaskStatus, error) {
	task, err := s.taskRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	status := &scheduler.TaskStatus{
		Task: *task,
	}

	// Check if running
	s.runningMu.RLock()
	for _, rt := range s.running {
		if rt.exec.TaskID == id {
			status.IsRunning = true
			break
		}
	}
	s.runningMu.RUnlock()

	// Get last execution
	lastExec, err := s.execRepo.GetLatest(ctx, id)
	if err == nil {
		status.LastRun = lastExec.EndedAt
		status.LastSuccess = lastExec.Success
		status.LastError = lastExec.Error
	}

	// Get next run from cron
	s.mu.RLock()
	if cronID, ok := s.cronIDs[id]; ok {
		entry := s.cron.Entry(cronID)
		if !entry.Next.IsZero() {
			next := entry.Next
			status.NextRun = &next
		}
	}
	s.mu.RUnlock()

	return status, nil
}

// ListTaskStatuses returns all tasks with runtime status.
func (s *Service) ListTaskStatuses(ctx context.Context) ([]*scheduler.TaskStatus, error) {
	tasks, err := s.taskRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]*scheduler.TaskStatus, len(tasks))
	for i, task := range tasks {
		status, err := s.GetTaskStatus(ctx, task.ID)
		if err != nil {
			// Return basic status on error
			status = &scheduler.TaskStatus{Task: *task}
		}
		statuses[i] = status
	}

	return statuses, nil
}

// UpdateTask updates a task's schedule or enabled state.
func (s *Service) UpdateTask(ctx context.Context, id string, update scheduler.TaskUpdate) error {
	if err := s.taskRepo.Update(ctx, id, update); err != nil {
		return err
	}

	// Reload and reschedule if schedule changed
	task, err := s.taskRepo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Remove old cron entry
	s.mu.Lock()
	if cronID, ok := s.cronIDs[id]; ok {
		s.cron.Remove(cronID)
		delete(s.cronIDs, id)
	}
	s.mu.Unlock()

	// Reschedule if enabled
	if task.Enabled && task.Schedule != "" {
		return s.scheduleTask(task)
	}

	return nil
}

// EnableTask enables a task.
func (s *Service) EnableTask(ctx context.Context, id string) error {
	enabled := true
	return s.UpdateTask(ctx, id, scheduler.TaskUpdate{Enabled: &enabled})
}

// DisableTask disables a task.
func (s *Service) DisableTask(ctx context.Context, id string) error {
	enabled := false
	return s.UpdateTask(ctx, id, scheduler.TaskUpdate{Enabled: &enabled})
}

// GetExecution retrieves an execution by ID.
func (s *Service) GetExecution(ctx context.Context, id string) (*scheduler.Execution, error) {
	return s.execRepo.Get(ctx, id)
}

// ListExecutions retrieves executions with filtering.
func (s *Service) ListExecutions(ctx context.Context, opts scheduler.ExecutionListOptions) ([]*scheduler.Execution, error) {
	return s.execRepo.List(ctx, opts)
}

// GetExecutionHistory retrieves recent executions for a task.
func (s *Service) GetExecutionHistory(ctx context.Context, taskID string, limit int) ([]*scheduler.Execution, error) {
	return s.execRepo.List(ctx, scheduler.ExecutionListOptions{
		TaskID: taskID,
		Limit:  limit,
	})
}

// CancelExecution cancels a running execution.
func (s *Service) CancelExecution(ctx context.Context, executionID string) error {
	s.runningMu.Lock()
	rt, ok := s.running[executionID]
	if !ok {
		s.runningMu.Unlock()
		return fmt.Errorf("execution %s not running", executionID)
	}
	rt.cancel()
	s.runningMu.Unlock()
	return nil
}

// GetRunningExecutions returns all currently running executions.
func (s *Service) GetRunningExecutions(ctx context.Context) ([]*scheduler.Execution, error) {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	execs := make([]*scheduler.Execution, 0, len(s.running))
	for _, rt := range s.running {
		execs = append(execs, rt.exec)
	}
	return execs, nil
}
