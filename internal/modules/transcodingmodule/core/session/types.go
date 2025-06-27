// Package session provides types for session management
package session

import (
	"time"

	plugins "github.com/mantonx/viewra/sdk"
)

// Status represents the state of a transcoding session
type Status string

const (
	StatusPending  Status = "pending"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
	StatusStopped  Status = "stopped"
)

// Session represents an active transcoding session
type Session struct {
	ID        string
	Request   plugins.TranscodeRequest
	Handle    *plugins.TranscodeHandle
	Process   interface{} // Process information
	Status    Status
	Progress  float64
	StartTime time.Time
	Error     error
}
