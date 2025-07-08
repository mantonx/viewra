// Package types provides types and interfaces for the transcoding module.
package types

import (
	"time"

	plugins "github.com/mantonx/viewra/sdk"
)

// SessionInfo represents information about a transcoding session
type SessionInfo struct {
	SessionID   string                  `json:"sessionId"`
	MediaID     string                  `json:"mediaId"`
	Provider    string                  `json:"provider"`
	Container   string                  `json:"container"`
	Status      plugins.TranscodeStatus `json:"status"`
	Progress    float64                 `json:"progress"`
	StartTime   time.Time               `json:"startTime"`
	Directory   string                  `json:"directory"`
	ContentHash string                  `json:"contentHash,omitempty"`
	Error       string                  `json:"error,omitempty"`
}