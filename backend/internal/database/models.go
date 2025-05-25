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

// MediaLibrary represents a directory to scan for media files
type MediaLibrary struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null" json:"path"`
	Type      string    `gorm:"not null" json:"type"` // "movie" or "tv"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MediaLibraryRequest represents the request to create a new media library
type MediaLibraryRequest struct {
	Path string `json:"path" binding:"required"`
	Type string `json:"type" binding:"required,oneof=movie tv music"`
}

// MediaFile represents a scanned media file on disk
type MediaFile struct {
	ID         uint         `gorm:"primaryKey" json:"id"`
	Path       string       `gorm:"uniqueIndex;not null" json:"path"`
	Size       int64        `gorm:"not null" json:"size"`
	Hash       string       `gorm:"index" json:"hash"`
	LibraryID  uint         `gorm:"not null" json:"library_id"`
	Library    MediaLibrary `gorm:"foreignKey:LibraryID" json:"library,omitempty"`
	LastSeen   time.Time    `gorm:"not null" json:"last_seen"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	MusicMetadata *MusicMetadata `gorm:"foreignKey:MediaFileID" json:"music_metadata,omitempty"`
}

// MusicMetadata represents extracted metadata from music files
type MusicMetadata struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	MediaFileID uint          `gorm:"index;not null" json:"media_file_id"`
	Title       string        `json:"title"`
	Album       string        `json:"album"`
	Artist      string        `json:"artist"`
	AlbumArtist string        `json:"album_artist"`
	Genre       string        `json:"genre"`
	Year        int           `json:"year"`
	Track       int           `json:"track"`
	TrackTotal  int           `json:"track_total"`
	Disc        int           `json:"disc"`
	DiscTotal   int           `json:"disc_total"`
	Duration    time.Duration `json:"duration"`
	Bitrate     int           `json:"bitrate"`
	Format      string        `json:"format"`
	HasArtwork  bool          `json:"has_artwork"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ScanJob represents a background scanning job
type ScanJob struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	LibraryID uint      `gorm:"not null" json:"library_id"`
	Library   MediaLibrary `gorm:"foreignKey:LibraryID" json:"library,omitempty"`
	Status    string    `gorm:"not null;default:'pending'" json:"status"` // pending, running, completed, failed, paused
	Progress  int       `gorm:"default:0" json:"progress"` // 0-100
	FilesFound int      `gorm:"default:0" json:"files_found"`
	FilesProcessed int  `gorm:"default:0" json:"files_processed"`
	BytesProcessed int64 `gorm:"default:0" json:"bytes_processed"`
	ErrorMessage string `json:"error_message,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	ResumedAt *time.Time `json:"resumed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
