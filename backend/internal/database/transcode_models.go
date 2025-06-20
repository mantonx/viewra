package database

import (
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// TranscodeStatus represents the status of a transcoding session
type TranscodeStatus string

const (
	TranscodeStatusQueued    TranscodeStatus = "queued"
	TranscodeStatusRunning   TranscodeStatus = "running"
	TranscodeStatusCompleted TranscodeStatus = "completed"
	TranscodeStatusFailed    TranscodeStatus = "failed"
	TranscodeStatusCancelled TranscodeStatus = "cancelled"
)

// TranscodeSession represents a unified transcoding session for any provider
type TranscodeSession struct {
	gorm.Model
	ID            string                       `gorm:"primaryKey;type:varchar(128)"`
	Provider      string                       `gorm:"index;type:varchar(64);not null"`
	Status        TranscodeStatus              `gorm:"type:varchar(32);not null;index"`
	Request       *plugins.TranscodeRequest    `gorm:"type:jsonb"`
	Progress      *plugins.TranscodingProgress `gorm:"type:jsonb"`
	Result        *plugins.TranscodeResult     `gorm:"type:jsonb"`
	Hardware      *plugins.HardwareInfo        `gorm:"type:jsonb"`
	StartTime     time.Time                    `gorm:"not null;index"`
	EndTime       *time.Time                   `gorm:"index"`
	LastAccessed  time.Time                    `gorm:"not null;index"`
	DirectoryPath string                       `gorm:"type:varchar(512)"`

	// Indexes for efficient queries
	// Index on (provider, status) for provider-specific queries
	// Index on (status, last_accessed) for cleanup queries
}

// TableName returns the table name for GORM
func (TranscodeSession) TableName() string {
	return "transcode_sessions"
}
