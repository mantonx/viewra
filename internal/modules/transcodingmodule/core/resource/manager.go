// Package resource provides resource management for transcoding operations.
// It implements limits and queuing to prevent resource exhaustion.
package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
)

// Manager manages resource allocation for transcoding operations.
// It enforces concurrent session limits and provides queuing capabilities.
type Manager struct {
	maxConcurrentSessions int
	sessionTimeout        time.Duration
	logger                hclog.Logger

	// Active session tracking
	activeSessions     map[string]*SessionResource
	activeSessionMutex sync.RWMutex

	// Queue management
	sessionQueue       chan *QueuedRequest
	queueMutex         sync.RWMutex
	maxQueueSize       int

	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// SessionResource tracks resource usage for an active session
type SessionResource struct {
	SessionID   string
	MediaID     string
	Provider    string
	StartTime   time.Time
	LastUpdate  time.Time
	Status      plugins.TranscodeStatus
	Handle      *plugins.TranscodeHandle
	
	// Resource tracking
	EstimatedMemoryMB int64
	ProcessIDs        []int
}

// QueuedRequest represents a transcoding request waiting in queue
type QueuedRequest struct {
	Request   plugins.TranscodeRequest
	StartFunc func(context.Context, plugins.TranscodeRequest) (*plugins.TranscodeHandle, error)
	ResultCh  chan *QueuedResult
	Timeout   time.Duration
	QueuedAt  time.Time
}

// QueuedResult represents the result of a queued request
type QueuedResult struct {
	Handle *plugins.TranscodeHandle
	Error  error
}

// Config holds configuration for the resource manager
type Config struct {
	MaxConcurrentSessions int
	SessionTimeout        time.Duration
	MaxQueueSize          int
	QueueTimeout          time.Duration
}

// DefaultConfig returns default resource management configuration
func DefaultConfig() *Config {
	return &Config{
		MaxConcurrentSessions: 4,
		SessionTimeout:        2 * time.Hour,
		MaxQueueSize:          20,
		QueueTimeout:          10 * time.Minute,
	}
}

// NewManager creates a new resource manager
func NewManager(config *Config, logger hclog.Logger) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	rm := &Manager{
		maxConcurrentSessions: config.MaxConcurrentSessions,
		sessionTimeout:        config.SessionTimeout,
		maxQueueSize:          config.MaxQueueSize,
		logger:                logger.Named("resource-manager"),
		activeSessions:        make(map[string]*SessionResource),
		sessionQueue:          make(chan *QueuedRequest, config.MaxQueueSize),
		ctx:                   ctx,
		cancel:                cancel,
	}

	// Start background workers
	rm.wg.Add(2)
	go rm.queueProcessor()
	go rm.sessionMonitor()

	return rm
}

// StartTranscode attempts to start a transcoding session with resource management.
// If resource limits are exceeded, the request is queued.
func (rm *Manager) StartTranscode(
	ctx context.Context,
	req plugins.TranscodeRequest,
	startFunc func(context.Context, plugins.TranscodeRequest) (*plugins.TranscodeHandle, error),
) (*plugins.TranscodeHandle, error) {
	// Check if we have capacity for immediate execution
	if rm.hasCapacity() {
		handle, err := rm.executeTranscode(ctx, req, startFunc)
		if err == nil {
			return handle, nil
		}
		// If execution failed, continue to queue (might be temporary failure)
		rm.logger.Warn("Direct execution failed, queuing request", "error", err)
	}

	// Queue the request
	rm.logger.Info("Queuing transcoding request due to resource limits",
		"active_sessions", rm.getActiveSessionCount(),
		"max_concurrent", rm.maxConcurrentSessions,
		"media_id", req.MediaID)

	return rm.queueTranscode(ctx, req, startFunc)
}

// hasCapacity checks if we have capacity for a new session
func (rm *Manager) hasCapacity() bool {
	rm.activeSessionMutex.RLock()
	defer rm.activeSessionMutex.RUnlock()
	return len(rm.activeSessions) < rm.maxConcurrentSessions
}

// executeTranscode executes a transcoding request immediately
func (rm *Manager) executeTranscode(
	ctx context.Context,
	req plugins.TranscodeRequest,
	startFunc func(context.Context, plugins.TranscodeRequest) (*plugins.TranscodeHandle, error),
) (*plugins.TranscodeHandle, error) {
	handle, err := startFunc(ctx, req)
	if err != nil {
		return nil, err
	}

	// Track the session
	resource := &SessionResource{
		SessionID:         handle.SessionID,
		MediaID:           req.MediaID,
		Provider:          handle.Provider,
		StartTime:         time.Now(),
		LastUpdate:        time.Now(),
		Status:            handle.Status,
		Handle:            handle,
		EstimatedMemoryMB: rm.estimateMemoryUsage(req),
	}

	rm.activeSessionMutex.Lock()
	rm.activeSessions[handle.SessionID] = resource
	rm.activeSessionMutex.Unlock()

	rm.logger.Info("Started transcoding session with resource tracking",
		"session_id", handle.SessionID,
		"active_sessions", len(rm.activeSessions),
		"estimated_memory_mb", resource.EstimatedMemoryMB)

	return handle, nil
}

// queueTranscode queues a transcoding request for later execution
func (rm *Manager) queueTranscode(
	ctx context.Context,
	req plugins.TranscodeRequest,
	startFunc func(context.Context, plugins.TranscodeRequest) (*plugins.TranscodeHandle, error),
) (*plugins.TranscodeHandle, error) {
	queuedReq := &QueuedRequest{
		Request:   req,
		StartFunc: startFunc,
		ResultCh:  make(chan *QueuedResult, 1),
		Timeout:   10 * time.Minute, // Queue timeout
		QueuedAt:  time.Now(),
	}

	// Try to add to queue
	select {
	case rm.sessionQueue <- queuedReq:
		rm.logger.Debug("Request queued successfully", "media_id", req.MediaID)
	default:
		return nil, fmt.Errorf("transcoding queue is full (max %d)", rm.maxQueueSize)
	}

	// Wait for result
	select {
	case result := <-queuedReq.ResultCh:
		return result.Handle, result.Error
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(queuedReq.Timeout):
		return nil, fmt.Errorf("request timed out in queue after %v", queuedReq.Timeout)
	}
}

// queueProcessor processes queued requests in background
func (rm *Manager) queueProcessor() {
	defer rm.wg.Done()

	rm.logger.Info("Queue processor started")

	for {
		select {
		case <-rm.ctx.Done():
			rm.logger.Info("Queue processor stopping")
			return

		case queuedReq := <-rm.sessionQueue:
			rm.processQueuedRequest(queuedReq)
		}
	}
}

// processQueuedRequest processes a single queued request
func (rm *Manager) processQueuedRequest(queuedReq *QueuedRequest) {
	// Check if request has timed out
	if time.Since(queuedReq.QueuedAt) > queuedReq.Timeout {
		queuedReq.ResultCh <- &QueuedResult{
			Error: fmt.Errorf("request expired in queue"),
		}
		return
	}

	// Wait for capacity
	for !rm.hasCapacity() {
		select {
		case <-rm.ctx.Done():
			queuedReq.ResultCh <- &QueuedResult{
				Error: fmt.Errorf("resource manager shutting down"),
			}
			return
		case <-time.After(1 * time.Second):
			// Check again
		}
	}

	// Execute the request
	ctx, cancel := context.WithTimeout(rm.ctx, rm.sessionTimeout)
	defer cancel()

	handle, err := rm.executeTranscode(ctx, queuedReq.Request, queuedReq.StartFunc)

	queuedReq.ResultCh <- &QueuedResult{
		Handle: handle,
		Error:  err,
	}
}

// sessionMonitor monitors active sessions and cleans up completed/failed ones
func (rm *Manager) sessionMonitor() {
	defer rm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	rm.logger.Info("Session monitor started")

	for {
		select {
		case <-rm.ctx.Done():
			rm.logger.Info("Session monitor stopping")
			return

		case <-ticker.C:
			rm.cleanupSessions()
		}
	}
}

// cleanupSessions removes completed, failed, or timed-out sessions
func (rm *Manager) cleanupSessions() {
	rm.activeSessionMutex.Lock()
	defer rm.activeSessionMutex.Unlock()

	now := time.Now()
	removed := 0

	for sessionID, resource := range rm.activeSessions {
		shouldRemove := false

		// Check for timeout
		if now.Sub(resource.StartTime) > rm.sessionTimeout {
			rm.logger.Warn("Session timed out, removing from tracking",
				"session_id", sessionID,
				"duration", now.Sub(resource.StartTime))
			shouldRemove = true
		}

		// Check status (this would need provider integration to get current status)
		if resource.Status == "completed" || resource.Status == "failed" || resource.Status == "cancelled" {
			rm.logger.Debug("Session completed, removing from tracking",
				"session_id", sessionID,
				"status", resource.Status)
			shouldRemove = true
		}

		if shouldRemove {
			delete(rm.activeSessions, sessionID)
			removed++
		}
	}

	if removed > 0 {
		rm.logger.Info("Cleaned up sessions",
			"removed", removed,
			"active_remaining", len(rm.activeSessions))
	}
}

// UpdateSessionStatus updates the status of a tracked session
func (rm *Manager) UpdateSessionStatus(sessionID string, status plugins.TranscodeStatus) {
	rm.activeSessionMutex.Lock()
	defer rm.activeSessionMutex.Unlock()

	if resource, exists := rm.activeSessions[sessionID]; exists {
		resource.Status = status
		resource.LastUpdate = time.Now()

		// Remove completed sessions immediately
		if status == "completed" || status == "failed" || status == "cancelled" {
			delete(rm.activeSessions, sessionID)
			rm.logger.Debug("Session status updated and removed",
				"session_id", sessionID,
				"status", status)
		}
	}
}

// RemoveSession removes a session from tracking (e.g., when stopped)
func (rm *Manager) RemoveSession(sessionID string) {
	rm.activeSessionMutex.Lock()
	defer rm.activeSessionMutex.Unlock()

	if _, exists := rm.activeSessions[sessionID]; exists {
		delete(rm.activeSessions, sessionID)
		rm.logger.Debug("Session removed from tracking", "session_id", sessionID)
	}
}

// GetResourceUsage returns current resource usage statistics
func (rm *Manager) GetResourceUsage() *ResourceUsage {
	rm.activeSessionMutex.RLock()
	defer rm.activeSessionMutex.RUnlock()

	usage := &ResourceUsage{
		ActiveSessions:    len(rm.activeSessions),
		MaxSessions:       rm.maxConcurrentSessions,
		QueuedRequests:    len(rm.sessionQueue),
		MaxQueueSize:      rm.maxQueueSize,
		TotalMemoryMB:     0,
		SessionDetails:    make([]SessionSummary, 0, len(rm.activeSessions)),
	}

	for _, resource := range rm.activeSessions {
		usage.TotalMemoryMB += resource.EstimatedMemoryMB
		usage.SessionDetails = append(usage.SessionDetails, SessionSummary{
			SessionID:         resource.SessionID,
			MediaID:           resource.MediaID,
			Provider:          resource.Provider,
			Status:            string(resource.Status),
			Duration:          time.Since(resource.StartTime),
			EstimatedMemoryMB: resource.EstimatedMemoryMB,
		})
	}

	return usage
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	ActiveSessions    int              `json:"active_sessions"`
	MaxSessions       int              `json:"max_sessions"`
	QueuedRequests    int              `json:"queued_requests"`
	MaxQueueSize      int              `json:"max_queue_size"`
	TotalMemoryMB     int64            `json:"total_memory_mb"`
	SessionDetails    []SessionSummary `json:"session_details"`
}

// SessionSummary provides summary info about an active session
type SessionSummary struct {
	SessionID         string        `json:"session_id"`
	MediaID           string        `json:"media_id"`
	Provider          string        `json:"provider"`
	Status            string        `json:"status"`
	Duration          time.Duration `json:"duration"`
	EstimatedMemoryMB int64         `json:"estimated_memory_mb"`
}

// estimateMemoryUsage estimates memory usage for a transcoding request
func (rm *Manager) estimateMemoryUsage(req plugins.TranscodeRequest) int64 {
	// Simple estimation based on resolution and codec
	// This is a rough estimate and could be improved with actual monitoring
	
	baseMemory := int64(200) // Base memory in MB
	
	// Add memory based on resolution
	if req.Resolution != nil {
		pixels := int64(req.Resolution.Width * req.Resolution.Height)
		// ~1MB per 100k pixels (rough estimate)
		resolutionMemory := pixels / 100000
		baseMemory += resolutionMemory
	}
	
	// Add memory based on codec complexity
	if req.VideoCodec == "h265" || req.VideoCodec == "av1" {
		baseMemory *= 2 // More complex codecs use more memory
	}
	
	return baseMemory
}

// getActiveSessionCount returns the current number of active sessions
func (rm *Manager) getActiveSessionCount() int {
	rm.activeSessionMutex.RLock()
	defer rm.activeSessionMutex.RUnlock()
	return len(rm.activeSessions)
}

// Shutdown gracefully shuts down the resource manager
func (rm *Manager) Shutdown() {
	rm.logger.Info("Shutting down resource manager")
	rm.cancel()
	rm.wg.Wait()
	close(rm.sessionQueue)
	rm.logger.Info("Resource manager shutdown complete")
}