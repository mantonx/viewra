package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/scheduler"
	"github.com/mantonx/viewra/internal/version"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db             *sql.DB
	scheduler      *scheduler.Scheduler
	transcodeQueue *transcode.Queue
	startTime      time.Time
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(db *sql.DB, scheduler *scheduler.Scheduler, transcodeQueue *transcode.Queue) *HealthHandler {
	return &HealthHandler{
		db:             db,
		scheduler:      scheduler,
		transcodeQueue: transcodeQueue,
		startTime:      time.Now(),
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status     string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Version    string            `json:"version"`
	Uptime     string            `json:"uptime"`
	Timestamp  time.Time         `json:"timestamp"`
	Components map[string]Check  `json:"components"`
	System     *SystemInfo       `json:"system,omitempty"`
}

// Check represents a component health check
type Check struct {
	Status  string `json:"status"` // "pass", "fail", "warn"
	Message string `json:"message,omitempty"`
}

// SystemInfo represents system resource information
type SystemInfo struct {
	NumGoroutines int    `json:"num_goroutines"`
	MemoryUsageMB uint64 `json:"memory_usage_mb"`
	NumCPU        int    `json:"num_cpu"`
}

// DatabaseHealth represents database health status (deprecated)
type DatabaseHealth struct {
	Status string `json:"status"`
	Ping   string `json:"ping,omitempty"`
}

// Check godoc
// @Summary Health check endpoint
// @Description Returns comprehensive health status of the application and its dependencies
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	components := make(map[string]Check)
	overallStatus := "healthy"

	// Check database
	dbCheck := h.checkDatabase(ctx)
	components["database"] = dbCheck
	if dbCheck.Status == "fail" {
		overallStatus = "unhealthy"
	} else if dbCheck.Status == "warn" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// Check scheduler (if available)
	if h.scheduler != nil {
		schedulerCheck := h.checkScheduler()
		components["scheduler"] = schedulerCheck
		if schedulerCheck.Status == "fail" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}

	// Check transcode queue (if available)
	if h.transcodeQueue != nil {
		queueCheck := h.checkTranscodeQueue()
		components["transcode_queue"] = queueCheck
		if queueCheck.Status == "fail" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}

	// Check disk space
	diskCheck := h.checkDiskSpace()
	components["disk_space"] = diskCheck
	if diskCheck.Status == "warn" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// Build response
	response := HealthResponse{
		Status:     overallStatus,
		Version:    version.Info(),
		Uptime:     time.Since(h.startTime).String(),
		Timestamp:  time.Now().UTC(),
		Components: components,
		System:     h.getSystemInfo(),
	}

	// Return appropriate HTTP status
	if overallStatus == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, response)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// checkDatabase checks database connectivity
func (h *HealthHandler) checkDatabase(ctx context.Context) Check {
	start := time.Now()

	if err := h.db.PingContext(ctx); err != nil {
		return Check{
			Status:  "fail",
			Message: "Database unreachable: " + err.Error(),
		}
	}

	latency := time.Since(start)
	if latency > 100*time.Millisecond {
		return Check{
			Status:  "warn",
			Message: "Database latency high: " + latency.String(),
		}
	}

	return Check{
		Status:  "pass",
		Message: "Ping: " + latency.String(),
	}
}

// checkScheduler checks scheduler status
func (h *HealthHandler) checkScheduler() Check {
	// For now, just check if scheduler exists
	// Could be enhanced to check if tasks are running
	if h.scheduler == nil {
		return Check{
			Status:  "fail",
			Message: "Scheduler not initialized",
		}
	}

	return Check{
		Status:  "pass",
		Message: "Scheduler operational",
	}
}

// checkTranscodeQueue checks transcode queue status
func (h *HealthHandler) checkTranscodeQueue() Check {
	if h.transcodeQueue == nil {
		return Check{
			Status:  "warn",
			Message: "Transcode queue not initialized",
		}
	}

	return Check{
		Status:  "pass",
		Message: "Queue operational",
	}
}

// checkDiskSpace checks available disk space
func (h *HealthHandler) checkDiskSpace() Check {
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		return Check{
			Status:  "warn",
			Message: "Cannot determine disk space",
		}
	}

	err = syscall.Statfs(wd, &stat)
	if err != nil {
		return Check{
			Status:  "warn",
			Message: "Cannot check disk space: " + err.Error(),
		}
	}

	// Calculate available space in GB
	availableGB := (stat.Bavail * uint64(stat.Bsize)) / (1024 * 1024 * 1024)

	if availableGB < 1 {
		return Check{
			Status:  "warn",
			Message: "Low disk space: less than 1GB available",
		}
	}

	return Check{
		Status:  "pass",
		Message: "",
	}
}

// getSystemInfo retrieves system resource information
func (h *HealthHandler) getSystemInfo() *SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &SystemInfo{
		NumGoroutines: runtime.NumGoroutine(),
		MemoryUsageMB: memStats.Alloc / 1024 / 1024,
		NumCPU:        runtime.NumCPU(),
	}
}
