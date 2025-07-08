// Package streaming handles streaming-related operations for playback
package streaming

import (
	"github.com/hashicorp/go-hclog"
)

// Manager orchestrates streaming operations
type Manager struct {
	logger           hclog.Logger
	progressHandler  *ProgressiveHandler
}

// NewManager creates a new streaming manager
func NewManager(logger hclog.Logger) *Manager {
	return &Manager{
		logger:          logger,
		progressHandler: NewProgressiveHandler(logger.Named("progressive")),
	}
}

// GetProgressiveHandler returns the progressive handler
func (m *Manager) GetProgressiveHandler() *ProgressiveHandler {
	return m.progressHandler
}

// Note: The actual serving is handled by ServeFile which takes a gin.Context
// These methods are removed as they don't match the ProgressiveHandler interface