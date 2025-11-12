package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status   string            `json:"status"`
	Time     time.Time         `json:"time"`
	Database DatabaseHealth    `json:"database"`
	Checks   map[string]string `json:"checks,omitempty"`
}

// DatabaseHealth represents database health status
type DatabaseHealth struct {
	Status string `json:"status"`
	Ping   string `json:"ping,omitempty"`
}

// Check godoc
// @Summary Health check endpoint
// @Description Returns the health status of the application and its dependencies
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	response := HealthResponse{
		Status: "ok",
		Time:   time.Now().UTC(),
		Checks: make(map[string]string),
	}

	// Check database connectivity
	dbStatus := h.checkDatabase(ctx)
	response.Database = dbStatus

	// If database is down, overall status is degraded
	if dbStatus.Status != "ok" {
		response.Status = "degraded"
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// checkDatabase checks database connectivity
func (h *HealthHandler) checkDatabase(ctx context.Context) DatabaseHealth {
	start := time.Now()

	if err := h.db.PingContext(ctx); err != nil {
		return DatabaseHealth{
			Status: "error",
			Ping:   err.Error(),
		}
	}

	latency := time.Since(start)

	return DatabaseHealth{
		Status: "ok",
		Ping:   latency.String(),
	}
}
