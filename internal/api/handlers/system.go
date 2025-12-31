package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/app/lifecycle"
	"github.com/mantonx/viewra/internal/application/system"
	"github.com/mantonx/viewra/internal/application/system/migration"
)

// SystemHandler handles system-level HTTP requests.
type SystemHandler struct {
	lifecycleMgr   *lifecycle.Manager
	maintenanceMgr *system.MaintenanceManager
	migrationSvc   *migration.Service
	db             *sql.DB // for health checks in SSE status events
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(lifecycleMgr *lifecycle.Manager, db *sql.DB) *SystemHandler {
	maintenanceMgr := system.NewMaintenanceManager()
	return &SystemHandler{
		lifecycleMgr:   lifecycleMgr,
		maintenanceMgr: maintenanceMgr,
		db:             db,
	}
}

// isReady checks if the server is fully operational (database connected, not in maintenance).
func (h *SystemHandler) isReady(ctx context.Context) bool {
	// Check maintenance mode
	if h.isMaintenanceMode() {
		return false
	}
	// Check database connectivity
	if h.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		return false
	}
	return true
}

// isMaintenanceMode returns true if maintenance mode is enabled.
func (h *SystemHandler) isMaintenanceMode() bool {
	return h.maintenanceMgr != nil && h.maintenanceMgr.IsEnabled()
}

// SetMigrationService sets the migration service with a config saver for updating database settings.
func (h *SystemHandler) SetMigrationService(db *sql.DB, driver string, configSaver migration.ConfigSaver) {
	h.migrationSvc = migration.NewService(db, driver, h.maintenanceMgr, configSaver)
}

// GetMaintenanceManager returns the maintenance manager for use by middleware.
func (h *SystemHandler) GetMaintenanceManager() *system.MaintenanceManager {
	return h.maintenanceMgr
}

// RestartRequest is the request body for triggering a restart.
type RestartRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RestartResponse is the response for restart-related endpoints.
type RestartResponse struct {
	Pending         bool     `json:"pending"`
	Reason          string   `json:"reason,omitempty"`
	RequestedAt     string   `json:"requested_at,omitempty"`
	PendingSettings []string `json:"pending_settings,omitempty"`
	Message         string   `json:"message,omitempty"`
}

// RequestRestart handles POST /api/admin/system/restart
// @Summary Request server restart
// @Description Requests the server to restart (admin only). The restart is graceful.
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body RestartRequest true "Restart reason"
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/restart [post]
func (h *SystemHandler) RequestRestart(c *gin.Context) {
	var req RestartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req.Reason = "Manual restart requested via API"
	}
	if req.Reason == "" {
		req.Reason = "Manual restart requested via API"
	}

	h.lifecycleMgr.RequestRestart(req.Reason)

	status := h.lifecycleMgr.GetRestartStatus()
	c.JSON(http.StatusOK, RestartResponse{
		Pending:         status.Pending,
		Reason:          status.Reason,
		RequestedAt:     formatTimePtr(status.RequestedAt),
		PendingSettings: status.PendingSettings,
		Message:         "Restart scheduled. Server will restart shortly.",
	})
}

// CancelRestart handles DELETE /api/admin/system/restart
// @Summary Cancel pending restart
// @Description Cancels a pending server restart (admin only)
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError "No pending restart"
// @Router /api/admin/system/restart [delete]
func (h *SystemHandler) CancelRestart(c *gin.Context) {
	if !h.lifecycleMgr.CancelRestart() {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "No pending restart to cancel")
		return
	}

	c.JSON(http.StatusOK, RestartResponse{
		Pending: false,
		Message: "Restart cancelled",
	})
}

// GetRestartStatus handles GET /api/admin/system/restart
// @Summary Get restart status
// @Description Returns the current restart status including pending settings
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Router /api/admin/system/restart [get]
func (h *SystemHandler) GetRestartStatus(c *gin.Context) {
	status := h.lifecycleMgr.GetRestartStatus()
	c.JSON(http.StatusOK, RestartResponse{
		Pending:         status.Pending,
		Reason:          status.Reason,
		RequestedAt:     formatTimePtr(status.RequestedAt),
		PendingSettings: status.PendingSettings,
	})
}

// ExecuteRestart handles POST /api/admin/system/restart/now
// @Summary Execute restart immediately
// @Description Triggers an immediate server restart (admin only). Use with caution.
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 202 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/restart/now [post]
func (h *SystemHandler) ExecuteRestart(c *gin.Context) {
	// First ensure a restart is requested
	if !h.lifecycleMgr.HasPendingRestart() {
		h.lifecycleMgr.RequestRestart("Immediate restart requested via API")
	}

	c.JSON(http.StatusAccepted, RestartResponse{
		Pending: true,
		Message: "Restart initiated. Server is shutting down.",
	})

	// Execute restart in background after response is sent
	go func() {
		time.Sleep(500 * time.Millisecond) // Allow response to be sent
		h.lifecycleMgr.ExecuteRestart(context.Background())
	}()
}

// AdminStatusEvent represents an SSE event for admin status updates.
type AdminStatusEvent struct {
	Type            string   `json:"type"`
	Ready           bool     `json:"ready"`       // Server is fully operational (db connected, not in maintenance)
	Maintenance     bool     `json:"maintenance"` // Server is in maintenance mode
	RestartPending  bool     `json:"restart_pending"`
	PendingSettings []string `json:"pending_settings,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	Timestamp       string   `json:"timestamp"`
}

// StreamAdminStatus handles GET /api/admin/status/stream
// @Summary Stream admin status updates
// @Description SSE stream of admin-relevant status updates (restart pending, etc.)
// @Tags system
// @Security BearerAuth
// @Produce text/event-stream
// @Success 200 {object} AdminStatusEvent
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/status/stream [get]
func (h *SystemHandler) StreamAdminStatus(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Send initial status
	status := h.lifecycleMgr.GetRestartStatus()
	event := AdminStatusEvent{
		Type:            "status",
		Ready:           h.isReady(c.Request.Context()),
		Maintenance:     h.isMaintenanceMode(),
		RestartPending:  status.Pending,
		PendingSettings: status.PendingSettings,
		Reason:          status.Reason,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeSSEEvent(c, "status", event); err != nil {
		return
	}
	c.Writer.Flush()

	// Create a ticker for periodic updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Stream updates
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.lifecycleMgr.ShutdownCh():
			// Server is shutting down
			event := AdminStatusEvent{
				Type:           "shutdown",
				Ready:          false,
				Maintenance:    false,
				RestartPending: true,
				Reason:         "Server is shutting down",
				Timestamp:      time.Now().UTC().Format(time.RFC3339),
			}
			_ = writeSSEEvent(c, "shutdown", event)
			c.Writer.Flush()
			return
		case <-ticker.C:
			// Send periodic status update
			status := h.lifecycleMgr.GetRestartStatus()
			event := AdminStatusEvent{
				Type:            "status",
				Ready:           h.isReady(c.Request.Context()),
				Maintenance:     h.isMaintenanceMode(),
				RestartPending:  status.Pending,
				PendingSettings: status.PendingSettings,
				Reason:          status.Reason,
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
			}
			if err := writeSSEEvent(c, "status", event); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

// writeSSEEvent writes an SSE event to the response.
func writeSSEEvent(c *gin.Context, eventType string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	return err
}

// formatTimePtr formats a time.Time, returning empty string for zero time.
func formatTimePtr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// MaintenanceStateResponse is the response for maintenance status.
type MaintenanceStateResponse struct {
	Enabled      bool   `json:"enabled"`
	Reason       string `json:"reason,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
	EstimatedEnd string `json:"estimatedEnd,omitempty"`
}

// MaintenanceRequest is the request to enable/disable maintenance mode.
type MaintenanceRequest struct {
	Enabled           bool   `json:"enabled"`
	Reason            string `json:"reason,omitempty"`
	EstimatedDuration string `json:"estimatedDuration,omitempty"` // Duration string, e.g. "30m"
}

// GetMaintenance handles GET /api/admin/system/maintenance
// @Summary Get maintenance mode status
// @Description Returns the current maintenance mode status
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MaintenanceStateResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/maintenance [get]
func (h *SystemHandler) GetMaintenance(c *gin.Context) {
	state := h.maintenanceMgr.GetState()
	c.JSON(http.StatusOK, h.stateToResponse(state))
}

// SetMaintenance handles POST /api/admin/system/maintenance
// @Summary Enable or disable maintenance mode
// @Description Enables or disables maintenance mode (admin only)
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body MaintenanceRequest true "Maintenance settings"
// @Success 200 {object} MaintenanceStateResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/maintenance [post]
func (h *SystemHandler) SetMaintenance(c *gin.Context) {
	var req MaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var state system.MaintenanceState
	if req.Enabled {
		// Parse estimated duration if provided
		var duration time.Duration
		if req.EstimatedDuration != "" {
			var err error
			duration, err = time.ParseDuration(req.EstimatedDuration)
			if err != nil {
				respondError(c, http.StatusBadRequest, "INVALID_DURATION", "Invalid duration format")
				return
			}
		}
		state = h.maintenanceMgr.Enable(req.Reason, duration)
	} else {
		state = h.maintenanceMgr.Disable()
	}

	c.JSON(http.StatusOK, h.stateToResponse(state))
}

// stateToResponse converts MaintenanceState to MaintenanceStateResponse.
func (h *SystemHandler) stateToResponse(state system.MaintenanceState) MaintenanceStateResponse {
	resp := MaintenanceStateResponse{
		Enabled: state.Enabled,
		Reason:  state.Reason,
	}
	if state.StartedAt != nil {
		resp.StartedAt = state.StartedAt.UTC().Format(time.RFC3339)
	}
	if state.EstimatedEnd != nil {
		resp.EstimatedEnd = state.EstimatedEnd.UTC().Format(time.RFC3339)
	}
	return resp
}

// Database Migration Endpoints

// TestDatabaseConnection handles POST /api/admin/system/database/test-connection
// @Summary Test database connection
// @Description Tests connection to a target database for migration
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body migration.TargetConfig true "Target database configuration"
// @Success 200 {object} migration.ConnectionTestResult
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/database/test-connection [post]
func (h *SystemHandler) TestDatabaseConnection(c *gin.Context) {
	if h.migrationSvc == nil {
		respondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Migration service not available")
		return
	}

	var config migration.TargetConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result := h.migrationSvc.TestConnection(c.Request.Context(), config)
	c.JSON(http.StatusOK, result)
}

// EstimateMigration handles POST /api/admin/system/database/estimate
// @Summary Estimate migration
// @Description Estimates migration time and data size
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body migration.EstimateRequest true "Estimate request"
// @Success 200 {object} migration.EstimateResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/database/estimate [post]
func (h *SystemHandler) EstimateMigration(c *gin.Context) {
	if h.migrationSvc == nil {
		respondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Migration service not available")
		return
	}

	var req migration.EstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.migrationSvc.Estimate(c.Request.Context(), req.TargetDriver)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "ESTIMATE_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, result)
}

// StartMigration handles POST /api/admin/system/database/migrate
// @Summary Start database migration
// @Description Starts a database migration to a new target
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body migration.MigrationRequest true "Migration request"
// @Success 200 {object} migration.MigrationStartResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/database/migrate [post]
func (h *SystemHandler) StartMigration(c *gin.Context) {
	if h.migrationSvc == nil {
		respondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Migration service not available")
		return
	}

	var req migration.MigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.migrationSvc.Start(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "MIGRATION_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetMigrationStatus handles GET /api/admin/system/database/migrate
// @Summary Get migration status
// @Description Returns the current migration status
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} migration.State
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/database/migrate [get]
func (h *SystemHandler) GetMigrationStatus(c *gin.Context) {
	if h.migrationSvc == nil {
		respondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Migration service not available")
		return
	}

	state := h.migrationSvc.GetState()
	c.JSON(http.StatusOK, state)
}
