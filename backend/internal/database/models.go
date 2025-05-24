package database

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Don't include password in JSON responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Media represents a media file in the system
type Media struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Filename    string    `gorm:"not null" json:"filename"`
	OriginalName string   `gorm:"not null" json:"original_name"`
	Path        string    `gorm:"not null" json:"path"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	Duration    *float64  `json:"duration,omitempty"` // For video/audio files
	Resolution  string    `json:"resolution,omitempty"` // For video/image files
	UserID      uint      `gorm:"not null" json:"user_id"`
	User        User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MediaFilter represents filters for media queries
type MediaFilter struct {
	UserID   *uint   `json:"user_id,omitempty"`
	MimeType *string `json:"mime_type,omitempty"`
	Limit    int     `json:"limit,omitempty"`
	Offset   int     `json:"offset,omitempty"`
}
