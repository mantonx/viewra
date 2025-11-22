package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterSchedulerRoutes registers scheduler admin routes
func RegisterSchedulerRoutes(router *gin.RouterGroup, handler *handlers.SchedulerHandler) {
	scheduler := router.Group("/scheduler")
	{
		// Task management
		scheduler.GET("/tasks", handler.ListTasks)
		scheduler.GET("/tasks/:id", handler.GetTaskStatus)
		scheduler.POST("/tasks/:id/trigger", handler.TriggerTask)
		scheduler.GET("/tasks/:id/history", handler.GetTaskHistory)
		scheduler.POST("/tasks/:id/enable", handler.EnableTask)
		scheduler.POST("/tasks/:id/disable", handler.DisableTask)
		scheduler.PUT("/tasks/:id/schedule", handler.UpdateTaskSchedule)
	}
}
