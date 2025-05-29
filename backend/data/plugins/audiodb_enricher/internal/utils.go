package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AudioDBCache represents cached API responses
type AudioDBCache struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SearchQuery string    `gorm:"uniqueIndex;size:255;not null" json:"search_query"`
	APIResponse string    `gorm:"type:longtext" json:"api_response"`
	CachedAt    time.Time `gorm:"not null" json:"cached_at"`
	ExpiresAt   time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AudioDBEnrichment represents enriched metadata for media files
type AudioDBEnrichment struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MediaFileID     uint      `gorm:"uniqueIndex;not null" json:"media_file_id"`
	AudioDBTrackID  string    `gorm:"index" json:"audiodb_track_id,omitempty"`
	AudioDBArtistID string    `gorm:"index" json:"audiodb_artist_id,omitempty"`
	AudioDBAlbumID  string    `gorm:"index" json:"audiodb_album_id,omitempty"`
	EnrichedTitle   string    `json:"enriched_title,omitempty"`
	EnrichedArtist  string    `json:"enriched_artist,omitempty"`
	EnrichedAlbum   string    `json:"enriched_album,omitempty"`
	EnrichedYear    int       `json:"enriched_year,omitempty"`
	EnrichedGenre   string    `json:"enriched_genre,omitempty"`
	MatchScore      float64   `json:"match_score"`
	ArtworkURL      string    `json:"artwork_url,omitempty"`
	ArtworkPath     string    `json:"artwork_path,omitempty"`
	BiographyURL    string    `json:"biography_url,omitempty"`
	EnrichedAt      time.Time `gorm:"not null" json:"enriched_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FileUtils provides utilities for file operations
type FileUtils struct{}

// NewFileUtils creates a new FileUtils instance
func NewFileUtils() *FileUtils {
	return &FileUtils{}
}

// IsSupportedAudioFile checks if the given file is a supported audio format
func (f *FileUtils) IsSupportedAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := []string{
		".mp3", ".flac", ".ogg", ".wav", ".aac", ".m4a", ".wma", ".opus", ".ape",
	}
	
	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	
	return false
}

// GetFileExtension returns the file extension without the dot
func (f *FileUtils) GetFileExtension(filePath string) string {
	ext := filepath.Ext(filePath)
	if len(ext) > 0 && ext[0] == '.' {
		return ext[1:]
	}
	return ext
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func (f *FileUtils) EnsureDirectoryExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// DatabaseUtils provides database utility functions
type DatabaseUtils struct {
	db *gorm.DB
}

// NewDatabaseUtils creates a new DatabaseUtils instance
func NewDatabaseUtils(db *gorm.DB) *DatabaseUtils {
	return &DatabaseUtils{db: db}
}

// MigrateTables ensures all plugin tables exist
func (d *DatabaseUtils) MigrateTables() error {
	return d.db.AutoMigrate(&AudioDBCache{}, &AudioDBEnrichment{})
}

// CleanExpiredCache removes expired cache entries
func (d *DatabaseUtils) CleanExpiredCache() error {
	return d.db.Where("expires_at < ?", time.Now()).Delete(&AudioDBCache{}).Error
}

// GetCacheStats returns cache statistics
func (d *DatabaseUtils) GetCacheStats() (map[string]interface{}, error) {
	var totalEntries int64
	var expiredEntries int64
	
	if err := d.db.Model(&AudioDBCache{}).Count(&totalEntries).Error; err != nil {
		return nil, fmt.Errorf("failed to count total cache entries: %w", err)
	}
	
	if err := d.db.Model(&AudioDBCache{}).Where("expires_at < ?", time.Now()).Count(&expiredEntries).Error; err != nil {
		return nil, fmt.Errorf("failed to count expired cache entries: %w", err)
	}
	
	return map[string]interface{}{
		"total_entries":   totalEntries,
		"active_entries":  totalEntries - expiredEntries,
		"expired_entries": expiredEntries,
	}, nil
}

// GetEnrichmentStats returns enrichment statistics
func (d *DatabaseUtils) GetEnrichmentStats() (map[string]interface{}, error) {
	var totalEnrichments int64
	var recentEnrichments int64
	
	if err := d.db.Model(&AudioDBEnrichment{}).Count(&totalEnrichments).Error; err != nil {
		return nil, fmt.Errorf("failed to count total enrichments: %w", err)
	}
	
	// Count enrichments from the last 24 hours
	yesterday := time.Now().Add(-24 * time.Hour)
	if err := d.db.Model(&AudioDBEnrichment{}).Where("enriched_at > ?", yesterday).Count(&recentEnrichments).Error; err != nil {
		return nil, fmt.Errorf("failed to count recent enrichments: %w", err)
	}
	
	return map[string]interface{}{
		"total_enrichments":  totalEnrichments,
		"recent_enrichments": recentEnrichments,
	}, nil
}

// StringUtils provides string manipulation utilities
type StringUtils struct{}

// NewStringUtils creates a new StringUtils instance
func NewStringUtils() *StringUtils {
	return &StringUtils{}
}

// NormalizeString normalizes a string for comparison
func (s *StringUtils) NormalizeString(str string) string {
	return strings.TrimSpace(strings.ToLower(str))
}

// RemoveSpecialCharacters removes special characters from a string
func (s *StringUtils) RemoveSpecialCharacters(str string) string {
	// Keep only alphanumeric characters and spaces
	var result strings.Builder
	for _, r := range str {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// TruncateString truncates a string to a maximum length
func (s *StringUtils) TruncateString(str string, maxLength int) string {
	if len(str) <= maxLength {
		return str
	}
	return str[:maxLength-3] + "..."
}

// IsEmpty checks if a string is empty or only whitespace
func (s *StringUtils) IsEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}

// CacheKeyBuilder builds consistent cache keys
type CacheKeyBuilder struct{}

// NewCacheKeyBuilder creates a new CacheKeyBuilder instance
func NewCacheKeyBuilder() *CacheKeyBuilder {
	return &CacheKeyBuilder{}
}

// BuildTrackSearchKey builds a cache key for track searches
func (c *CacheKeyBuilder) BuildTrackSearchKey(title, artist, album string) string {
	stringUtils := NewStringUtils()
	normalizedTitle := stringUtils.NormalizeString(title)
	normalizedArtist := stringUtils.NormalizeString(artist)
	normalizedAlbum := stringUtils.NormalizeString(album)
	
	return fmt.Sprintf("track:%s:%s:%s", normalizedTitle, normalizedArtist, normalizedAlbum)
}

// BuildArtistSearchKey builds a cache key for artist searches
func (c *CacheKeyBuilder) BuildArtistSearchKey(artistName string) string {
	stringUtils := NewStringUtils()
	normalized := stringUtils.NormalizeString(artistName)
	return fmt.Sprintf("artist:%s", normalized)
}

// BuildAlbumSearchKey builds a cache key for album searches
func (c *CacheKeyBuilder) BuildAlbumSearchKey(albumID string) string {
	return fmt.Sprintf("album:%s", albumID)
}

// Logger provides structured logging utilities
type Logger struct {
	prefix string
}

// NewLogger creates a new Logger instance
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// LogInfo logs an info message
func (l *Logger) LogInfo(message string, fields ...interface{}) {
	fmt.Printf("[INFO] %s: %s", l.prefix, fmt.Sprintf(message, fields...))
}

// LogError logs an error message
func (l *Logger) LogError(message string, fields ...interface{}) {
	fmt.Printf("[ERROR] %s: %s", l.prefix, fmt.Sprintf(message, fields...))
}

// LogDebug logs a debug message
func (l *Logger) LogDebug(message string, fields ...interface{}) {
	fmt.Printf("[DEBUG] %s: %s", l.prefix, fmt.Sprintf(message, fields...))
} 