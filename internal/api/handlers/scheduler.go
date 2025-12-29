package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	appscheduler "github.com/mantonx/viewra/internal/application/scheduler"
	"github.com/mantonx/viewra/internal/domain/scheduler"
)

// SchedulerHandler handles scheduler-related API requests
type SchedulerHandler struct {
	service *appscheduler.Service
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(s *appscheduler.Service) *SchedulerHandler {
	return &SchedulerHandler{
		service: s,
	}
}

// TaskResponse represents a task in API responses
type TaskResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Schedule    string  `json:"schedule"`
	Enabled     bool    `json:"enabled"`
	Source      string  `json:"source"`
	SourceID    string  `json:"source_id,omitempty"`
	LastRun     *string `json:"last_run,omitempty"`
	NextRun     *string `json:"next_run,omitempty"`
	LastError   string  `json:"last_error,omitempty"`
	LastSuccess *bool   `json:"last_success,omitempty"`
	IsRunning   bool    `json:"is_running"`
}

// ExecutionResponse represents an execution in API responses
type ExecutionResponse struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id"`
	Status      string  `json:"status"`
	TriggeredBy string  `json:"triggered_by"`
	StartedAt   *string `json:"started_at,omitempty"`
	EndedAt     *string `json:"ended_at,omitempty"`
	DurationMs  int64   `json:"duration_ms"`
	Success     *bool   `json:"success,omitempty"`
	Error       string  `json:"error,omitempty"`
	Attempt     int     `json:"attempt"`
}

// ListTasks returns all registered scheduled tasks
// @Summary List all scheduled tasks
// @Tags scheduler
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/scheduler/tasks [get]
func (h *SchedulerHandler) ListTasks(c *gin.Context) {
	statuses, err := h.service.ListTaskStatuses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list tasks",
		})
		return
	}

	tasks := make([]TaskResponse, len(statuses))
	for i, s := range statuses {
		tasks[i] = taskStatusToResponse(s)
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// GetTaskStatus returns status for a specific task
// @Summary Get task status
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} TaskResponse
// @Failure 404 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id} [get]
func (h *SchedulerHandler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	status, err := h.service.GetTaskStatus(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, taskStatusToResponse(status))
}

// TriggerTask manually triggers a task execution
// @Summary Trigger task execution
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id}/trigger [post]
func (h *SchedulerHandler) TriggerTask(c *gin.Context) {
	taskID := c.Param("id")

	exec, err := h.service.TriggerTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Task triggered successfully",
		"task_id":      taskID,
		"execution_id": exec.ID,
	})
}

// GetTaskHistory returns execution history for a task
// @Summary Get task execution history
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Param limit query int false "Maximum results" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id}/history [get]
func (h *SchedulerHandler) GetTaskHistory(c *gin.Context) {
	taskID := c.Param("id")

	// Parse limit query parameter (default: 10, max: 100)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
			if limit < 1 {
				limit = 10
			}
		}
	}

	executions, err := h.service.GetExecutionHistory(c.Request.Context(), taskID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task history",
		})
		return
	}

	history := make([]ExecutionResponse, len(executions))
	for i, e := range executions {
		history[i] = executionToResponse(e)
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"history": history,
		"count":   len(history),
	})
}

// EnableTask enables a disabled task
// @Summary Enable task
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id}/enable [post]
func (h *SchedulerHandler) EnableTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.service.EnableTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task enabled successfully",
		"task_id": taskID,
	})
}

// DisableTask disables an enabled task
// @Summary Disable task
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id}/disable [post]
func (h *SchedulerHandler) DisableTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.service.DisableTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task disabled successfully",
		"task_id": taskID,
	})
}

// UpdateScheduleRequest represents the request body for updating a task schedule
type UpdateScheduleRequest struct {
	Schedule string `json:"schedule" binding:"required"`
}

// UpdateTaskSchedule updates the schedule for a task
// @Summary Update task schedule
// @Tags scheduler
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param body body UpdateScheduleRequest true "New schedule"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/admin/scheduler/tasks/{id}/schedule [put]
func (h *SchedulerHandler) UpdateTaskSchedule(c *gin.Context) {
	taskID := c.Param("id")

	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	err := h.service.UpdateTask(c.Request.Context(), taskID, scheduler.TaskUpdate{
		Schedule: &req.Schedule,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Task schedule updated successfully",
		"task_id":  taskID,
		"schedule": req.Schedule,
	})
}

// CancelExecution cancels a running execution
// @Summary Cancel running execution
// @Tags scheduler
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/admin/scheduler/executions/{id}/cancel [post]
func (h *SchedulerHandler) CancelExecution(c *gin.Context) {
	execID := c.Param("id")

	err := h.service.CancelExecution(c.Request.Context(), execID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Execution cancelled",
		"execution_id": execID,
	})
}

// GetRunningExecutions returns all currently running executions
// @Summary List running executions
// @Tags scheduler
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/scheduler/executions/running [get]
func (h *SchedulerHandler) GetRunningExecutions(c *gin.Context) {
	executions, err := h.service.GetRunningExecutions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get running executions",
		})
		return
	}

	running := make([]ExecutionResponse, len(executions))
	for i, e := range executions {
		running[i] = executionToResponse(e)
	}

	c.JSON(http.StatusOK, gin.H{
		"executions": running,
		"count":      len(running),
	})
}

// --- Helper functions ---

func taskStatusToResponse(s *scheduler.TaskStatus) TaskResponse {
	resp := TaskResponse{
		ID:          s.Task.ID,
		Name:        s.Task.Name,
		Description: s.Task.Description,
		Schedule:    s.Task.Schedule,
		Enabled:     s.Task.Enabled,
		Source:      string(s.Task.Source),
		SourceID:    s.Task.SourceID,
		LastError:   s.LastError,
		LastSuccess: s.LastSuccess,
		IsRunning:   s.IsRunning,
	}

	if s.LastRun != nil {
		t := s.LastRun.Format("2006-01-02T15:04:05Z07:00")
		resp.LastRun = &t
	}
	if s.NextRun != nil {
		t := s.NextRun.Format("2006-01-02T15:04:05Z07:00")
		resp.NextRun = &t
	}

	return resp
}

func executionToResponse(e *scheduler.Execution) ExecutionResponse {
	resp := ExecutionResponse{
		ID:          e.ID,
		TaskID:      e.TaskID,
		Status:      string(e.Status),
		TriggeredBy: string(e.TriggeredBy),
		DurationMs:  e.DurationMs,
		Success:     e.Success,
		Error:       e.Error,
		Attempt:     e.Attempt,
	}

	if e.StartedAt != nil {
		t := e.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &t
	}
	if e.EndedAt != nil {
		t := e.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.EndedAt = &t
	}

	return resp
}
