package database

import (
	"encoding/json"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
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
	ID            string          `gorm:"primaryKey;type:varchar(128)"`
	Provider      string          `gorm:"index;type:varchar(64);not null"`
	Status        TranscodeStatus `gorm:"type:varchar(32);not null;index"`
	Request       string          `gorm:"type:text"` // JSON string
	Progress      string          `gorm:"type:text"` // JSON string
	Result        string          `gorm:"type:text"` // JSON string
	Hardware      string          `gorm:"type:text"` // JSON string
	StartTime     time.Time       `gorm:"not null;index"`
	EndTime       *time.Time      `gorm:"index"`
	LastAccessed  time.Time       `gorm:"not null;index"`
	DirectoryPath string          `gorm:"type:varchar(512)"`

	// Indexes for efficient queries
	// Index on (provider, status) for provider-specific queries
	// Index on (status, last_accessed) for cleanup queries
}

// TableName returns the table name for GORM
func (TranscodeSession) TableName() string {
	return "transcode_sessions"
}

// GetRequest deserializes the Request JSON string
func (t *TranscodeSession) GetRequest() (*plugins.TranscodeRequest, error) {
	if t.Request == "" {
		return nil, nil
	}
	var req plugins.TranscodeRequest
	if err := json.Unmarshal([]byte(t.Request), &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// GetProgress deserializes the Progress JSON string
func (t *TranscodeSession) GetProgress() (*plugins.TranscodingProgress, error) {
	if t.Progress == "" {
		return nil, nil
	}
	var prog plugins.TranscodingProgress
	if err := json.Unmarshal([]byte(t.Progress), &prog); err != nil {
		return nil, err
	}
	return &prog, nil
}

// GetResult deserializes the Result JSON string
func (t *TranscodeSession) GetResult() (*plugins.TranscodeResult, error) {
	if t.Result == "" {
		return nil, nil
	}
	var res plugins.TranscodeResult
	if err := json.Unmarshal([]byte(t.Result), &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetHardware deserializes the Hardware JSON string
func (t *TranscodeSession) GetHardware() (*plugins.HardwareInfo, error) {
	if t.Hardware == "" {
		return nil, nil
	}
	var hw plugins.HardwareInfo
	if err := json.Unmarshal([]byte(t.Hardware), &hw); err != nil {
		return nil, err
	}
	return &hw, nil
}

// SetProgress serializes and sets the Progress
func (t *TranscodeSession) SetProgress(prog *plugins.TranscodingProgress) error {
	if prog == nil {
		t.Progress = ""
		return nil
	}
	data, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	t.Progress = string(data)
	return nil
}

// SetResult serializes and sets the Result
func (t *TranscodeSession) SetResult(res *plugins.TranscodeResult) error {
	if res == nil {
		t.Result = ""
		return nil
	}
	data, err := json.Marshal(res)
	if err != nil {
		return err
	}
	t.Result = string(data)
	return nil
}

// SetHardware serializes and sets the Hardware
func (t *TranscodeSession) SetHardware(hw *plugins.HardwareInfo) error {
	if hw == nil {
		t.Hardware = ""
		return nil
	}
	data, err := json.Marshal(hw)
	if err != nil {
		return err
	}
	t.Hardware = string(data)
	return nil
}
