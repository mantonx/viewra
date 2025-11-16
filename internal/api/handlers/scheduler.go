package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/infrastructure/scheduler"
)

// SchedulerHandler handles scheduler-related API requests
type SchedulerHandler struct {
	scheduler *scheduler.Scheduler
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(s *scheduler.Scheduler) *SchedulerHandler {
	return &SchedulerHandler{
		scheduler: s,
	}
}

// ListTasks returns all registered scheduled tasks
// GET /api/admin/scheduler/tasks
func (h *SchedulerHandler) ListTasks(c *gin.Context) {
	tasks := h.scheduler.ListTasks()

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// GetTaskStatus returns status for a specific task
// GET /api/admin/scheduler/tasks/:id
func (h *SchedulerHandler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	status, err := h.scheduler.GetTaskStatus(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// TriggerTask manually triggers a task execution
// POST /api/admin/scheduler/tasks/:id/trigger
func (h *SchedulerHandler) TriggerTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.scheduler.TriggerNow(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Task triggered successfully",
		"task_id": taskID,
	})
}

// GetTaskHistory returns execution history for a task
// GET /api/admin/scheduler/tasks/:id/history?limit=10
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

	history, err := h.scheduler.GetExecutionHistory(c.Request.Context(), taskID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"history": history,
		"count":   len(history),
	})
}

// EnableTask enables a disabled task
// POST /api/admin/scheduler/tasks/:id/enable
func (h *SchedulerHandler) EnableTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.scheduler.EnableTask(taskID)
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
// POST /api/admin/scheduler/tasks/:id/disable
func (h *SchedulerHandler) DisableTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.scheduler.DisableTask(taskID)
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
// PUT /api/admin/scheduler/tasks/:id/schedule
func (h *SchedulerHandler) UpdateTaskSchedule(c *gin.Context) {
	taskID := c.Param("id")

	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	err := h.scheduler.UpdateTaskSchedule(taskID, req.Schedule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task schedule updated successfully",
		"task_id": taskID,
		"schedule": req.Schedule,
	})
}
