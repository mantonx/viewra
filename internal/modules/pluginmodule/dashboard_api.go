package pluginmodule

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	plugins "github.com/mantonx/viewra/sdk"
)

// DashboardAPIHandlers provides HTTP endpoints for dashboard functionality
type DashboardAPIHandlers struct {
	dashboardManager *DashboardManager
	wsUpgrader       websocket.Upgrader
	activeStreams    map[string]map[string]*websocket.Conn // sectionID -> clientID -> connection
	streamsMutex     sync.RWMutex
}

// WebSocketMessage represents a message sent via WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	SectionID string      `json:"section_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	DataType  string      `json:"data_type,omitempty"`
	Timestamp int64       `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// NewDashboardAPIHandlers creates new dashboard API handlers
func NewDashboardAPIHandlers(dashboardManager *DashboardManager) *DashboardAPIHandlers {
	handlers := &DashboardAPIHandlers{
		dashboardManager: dashboardManager,
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		activeStreams: make(map[string]map[string]*websocket.Conn),
	}

	// Start background update broadcaster
	go handlers.startUpdateBroadcaster()

	return handlers
}

// RegisterRoutes registers dashboard API routes
func (h *DashboardAPIHandlers) RegisterRoutes(router *gin.RouterGroup) {
	dashboard := router.Group("/dashboard")
	{
		// Dashboard discovery endpoints
		dashboard.GET("/sections", h.GetAllSections)
		dashboard.GET("/sections/types", h.GetSectionTypes)
		dashboard.GET("/sections/type/:type", h.GetSectionsByType)

		// Section data endpoints
		dashboard.GET("/sections/:sectionId/data/main", h.GetMainData)
		dashboard.GET("/sections/:sectionId/data/nerd", h.GetNerdData)
		dashboard.GET("/sections/:sectionId/data/metrics", h.GetMetrics)

		// Section management endpoints
		dashboard.POST("/sections/:sectionId/actions/:actionId", h.ExecuteAction)
		dashboard.POST("/sections/:sectionId/refresh", h.RefreshSection)

		// WebSocket endpoint
		dashboard.GET("/ws", h.HandleWebSocket)
	}
}

// HandleWebSocket handles WebSocket connections for real-time updates
func (h *DashboardAPIHandlers) HandleWebSocket(c *gin.Context) {
	conn, err := h.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to upgrade connection: %v", err),
		})
		return
	}

	defer conn.Close()

	// Generate unique client ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// Handle client subscription to sections
	h.handleWebSocketClient(conn, clientID)
}

// handleWebSocketClient manages a WebSocket client connection
func (h *DashboardAPIHandlers) handleWebSocketClient(conn *websocket.Conn, clientID string) {
	// Subscribe to all sections by default
	sections := h.dashboardManager.GetAllSections()

	h.streamsMutex.Lock()
	for _, section := range sections {
		if h.activeStreams[section.ID] == nil {
			h.activeStreams[section.ID] = make(map[string]*websocket.Conn)
		}
		h.activeStreams[section.ID][clientID] = conn
	}
	h.streamsMutex.Unlock()

	// Send initial data
	for _, section := range sections {
		h.sendSectionUpdate(section.ID, clientID)
	}

	// Keep connection alive and handle cleanup
	for {
		// Read message (we don't expect many from client, just keep-alive)
		_, _, err := conn.ReadMessage()
		if err != nil {
			// Client disconnected, clean up
			h.streamsMutex.Lock()
			for sectionID := range h.activeStreams {
				delete(h.activeStreams[sectionID], clientID)
				if len(h.activeStreams[sectionID]) == 0 {
					delete(h.activeStreams, sectionID)
				}
			}
			h.streamsMutex.Unlock()
			break
		}
	}
}

// sendSectionUpdate sends data for a specific section to a specific client
func (h *DashboardAPIHandlers) sendSectionUpdate(sectionID, clientID string) {
	h.streamsMutex.RLock()
	conn, exists := h.activeStreams[sectionID][clientID]
	h.streamsMutex.RUnlock()

	if !exists {
		return
	}

	// Get main data
	mainData, err := h.dashboardManager.GetMainData(context.Background(), sectionID)
	if err == nil {
		message := WebSocketMessage{
			Type:      "section_data",
			SectionID: sectionID,
			Data:      mainData,
			DataType:  "main",
			Timestamp: time.Now().Unix(),
		}
		h.sendMessageToClient(conn, message)
	}
}

// sendMessageToClient sends a message to a WebSocket client
func (h *DashboardAPIHandlers) sendMessageToClient(conn *websocket.Conn, message WebSocketMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.WriteMessage(websocket.TextMessage, data)
}

// broadcastToSection sends data to all clients subscribed to a section
func (h *DashboardAPIHandlers) broadcastToSection(sectionID string, message WebSocketMessage) {
	h.streamsMutex.RLock()
	clients := h.activeStreams[sectionID]
	h.streamsMutex.RUnlock()

	for clientID, conn := range clients {
		go func(c *websocket.Conn, cID string) {
			defer func() {
				if r := recover(); r != nil {
					// Client connection failed, remove it
					h.streamsMutex.Lock()
					delete(h.activeStreams[sectionID], cID)
					h.streamsMutex.Unlock()
				}
			}()
			h.sendMessageToClient(c, message)
		}(conn, clientID)
	}
}

// startUpdateBroadcaster starts a background goroutine that periodically broadcasts updates
func (h *DashboardAPIHandlers) startUpdateBroadcaster() {
	ticker := time.NewTicker(3 * time.Second) // Update every 3 seconds
	defer ticker.Stop()

	for range ticker.C {
		sections := h.dashboardManager.GetAllSections()

		for _, section := range sections {
			h.streamsMutex.RLock()
			hasClients := len(h.activeStreams[section.ID]) > 0
			h.streamsMutex.RUnlock()

			if !hasClients {
				continue
			}

			// Get updated data
			mainData, err := h.dashboardManager.GetMainData(context.Background(), section.ID)
			if err != nil {
				continue
			}

			// Broadcast to all clients
			message := WebSocketMessage{
				Type:      "section_data",
				SectionID: section.ID,
				Data:      mainData,
				DataType:  "main",
				Timestamp: time.Now().Unix(),
			}

			h.broadcastToSection(section.ID, message)
		}
	}
}

// GetAllSections returns all registered dashboard sections
func (h *DashboardAPIHandlers) GetAllSections(c *gin.Context) {
	sections := h.dashboardManager.GetAllSections()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sections,
		"total":   len(sections),
	})
}

// GetSectionTypes returns all unique section types
func (h *DashboardAPIHandlers) GetSectionTypes(c *gin.Context) {
	types := h.dashboardManager.GetSectionTypes()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types,
		"total":   len(types),
	})
}

// GetSectionsByType returns sections filtered by type
func (h *DashboardAPIHandlers) GetSectionsByType(c *gin.Context) {
	sectionType := c.Param("type")
	sections := h.dashboardManager.GetSectionsByType(sectionType)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sections,
		"total":   len(sections),
		"type":    sectionType,
	})
}

// GetMainData returns main data for a dashboard section
func (h *DashboardAPIHandlers) GetMainData(c *gin.Context) {
	sectionID := c.Param("sectionId")

	data, err := h.dashboardManager.GetMainData(c.Request.Context(), sectionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       data,
		"section_id": sectionID,
		"data_type":  "main",
		"timestamp":  time.Now().Unix(),
	})
}

// GetNerdData returns advanced/detailed data for a dashboard section
func (h *DashboardAPIHandlers) GetNerdData(c *gin.Context) {
	sectionID := c.Param("sectionId")

	data, err := h.dashboardManager.GetNerdData(c.Request.Context(), sectionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       data,
		"section_id": sectionID,
		"data_type":  "nerd",
		"timestamp":  time.Now().Unix(),
	})
}

// GetMetrics returns time-series metrics for a dashboard section
func (h *DashboardAPIHandlers) GetMetrics(c *gin.Context) {
	sectionID := c.Param("sectionId")

	// Parse query parameters for time range
	timeRange := plugins.TimeRange{
		End:  time.Now(),
		Step: "1m", // Default step
	}

	if startStr := c.Query("start"); startStr != "" {
		if startUnix, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			timeRange.Start = time.Unix(startUnix, 0)
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		if endUnix, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			timeRange.End = time.Unix(endUnix, 0)
		}
	}

	if step := c.Query("step"); step != "" {
		timeRange.Step = step
	}

	// Default to last hour if no start time specified
	if timeRange.Start.IsZero() {
		timeRange.Start = timeRange.End.Add(-time.Hour)
	}

	metrics, err := h.dashboardManager.GetMetrics(c.Request.Context(), sectionID, timeRange)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       metrics,
		"section_id": sectionID,
		"data_type":  "metrics",
		"time_range": timeRange,
		"timestamp":  time.Now().Unix(),
	})
}

// ExecuteAction executes a dashboard action
func (h *DashboardAPIHandlers) ExecuteAction(c *gin.Context) {
	sectionID := c.Param("sectionId")
	actionID := c.Param("actionId")

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		// If no JSON payload, use empty map
		payload = make(map[string]interface{})
	}

	result, err := h.dashboardManager.ExecuteAction(c.Request.Context(), sectionID, actionID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       result,
		"section_id": sectionID,
		"action_id":  actionID,
		"timestamp":  time.Now().Unix(),
	})
}

// RefreshSection forces a refresh of a dashboard section
func (h *DashboardAPIHandlers) RefreshSection(c *gin.Context) {
	sectionID := c.Param("sectionId")

	err := h.dashboardManager.RefreshSection(c.Request.Context(), sectionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Section refresh initiated",
		"section_id": sectionID,
		"timestamp":  time.Now().Unix(),
	})
}
