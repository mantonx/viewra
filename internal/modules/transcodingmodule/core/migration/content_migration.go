// Package migration provides services for migrating between session-based URLs
// and content-addressable URLs based on content hashes.
package migration

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// ContentMigrationService handles URL migration from session-based to content-addressable URLs
type ContentMigrationService struct {
	db     *gorm.DB
	logger hclog.Logger
}

// NewContentMigrationService creates a new content migration service
func NewContentMigrationService(db *gorm.DB) *ContentMigrationService {
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:  "content-migration",
		Level: hclog.Info,
	})

	return &ContentMigrationService{
		db:     db,
		logger: hclogger,
	}
}

// UpdateSessionContentHash updates a transcoding session with its content hash and URL
func (s *ContentMigrationService) UpdateSessionContentHash(sessionID, contentHash, contentURL string) error {
	s.logger.Info("Updating session with content hash",
		"sessionID", sessionID,
		"contentHash", contentHash,
		"contentURL", contentURL,
	)

	// Update the session record in the database
	result := s.db.Model(&database.TranscodeSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"content_hash":  contentHash,
			"last_accessed": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update session content hash: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	s.logger.Info("Session updated with content hash",
		"sessionID", sessionID,
		"contentHash", contentHash,
		"rowsAffected", result.RowsAffected,
	)

	return nil
}

// GetSessionContentHash retrieves the content hash for a session
func (s *ContentMigrationService) GetSessionContentHash(sessionID string) (string, error) {
	var session database.TranscodeSession
	result := s.db.Select("content_hash").Where("id = ?", sessionID).First(&session)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("session not found: %s", sessionID)
		}
		return "", fmt.Errorf("failed to get session content hash: %w", result.Error)
	}

	return session.ContentHash, nil
}

// ListSessionsWithoutContentHash returns sessions that have completed but don't have content hashes
func (s *ContentMigrationService) ListSessionsWithoutContentHash(limit int) ([]database.TranscodeSession, error) {
	var sessions []database.TranscodeSession

	query := s.db.Where("status = ? AND (content_hash = ? OR content_hash IS NULL)",
		database.TranscodeStatusCompleted, "").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&sessions)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list sessions without content hash: %w", result.Error)
	}

	return sessions, nil
}

// MigrateSessionToContentHash generates and updates content hash for a session
func (s *ContentMigrationService) MigrateSessionToContentHash(sessionID string) error {
	s.logger.Info("Migrating session to content hash", "sessionID", sessionID)

	// This would be called by the transcoding module when it needs to
	// retroactively generate content hashes for existing sessions
	// For now, we just log the intent - actual implementation would
	// regenerate the hash based on session parameters

	return fmt.Errorf("migration not yet implemented for session: %s", sessionID)
}

// GetContentHashStats returns statistics about content hash coverage
func (s *ContentMigrationService) GetContentHashStats() (*ContentHashStats, error) {
	stats := &ContentHashStats{}

	// Count total sessions
	result := s.db.Model(&database.TranscodeSession{}).Count(&stats.TotalSessions)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to count total sessions: %w", result.Error)
	}

	// Count sessions with content hash
	result = s.db.Model(&database.TranscodeSession{}).
		Where("content_hash != ? AND content_hash IS NOT NULL", "").
		Count(&stats.SessionsWithHash)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to count sessions with hash: %w", result.Error)
	}

	// Count completed sessions without content hash
	result = s.db.Model(&database.TranscodeSession{}).
		Where("status = ? AND (content_hash = ? OR content_hash IS NULL)",
			database.TranscodeStatusCompleted, "").
		Count(&stats.CompletedWithoutHash)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to count completed sessions without hash: %w", result.Error)
	}

	// Calculate percentages
	if stats.TotalSessions > 0 {
		stats.HashCoveragePercent = float64(stats.SessionsWithHash) / float64(stats.TotalSessions) * 100
	}

	return stats, nil
}

// ContentHashStats contains statistics about content hash coverage
type ContentHashStats struct {
	TotalSessions        int64   `json:"total_sessions"`
	SessionsWithHash     int64   `json:"sessions_with_hash"`
	CompletedWithoutHash int64   `json:"completed_without_hash"`
	HashCoveragePercent  float64 `json:"hash_coverage_percent"`
}

// CleanupOldSessions removes old session directories after content has been migrated
func (s *ContentMigrationService) CleanupOldSessions(olderThanDays int) error {
	if olderThanDays <= 0 {
		return fmt.Errorf("olderThanDays must be positive")
	}

	cutoffDate := time.Now().AddDate(0, 0, -olderThanDays)

	// Find sessions that are old, completed, and have content hashes
	var sessions []database.TranscodeSession
	result := s.db.Where("created_at < ? AND status = ? AND content_hash != ? AND content_hash IS NOT NULL",
		cutoffDate, database.TranscodeStatusCompleted, "").
		Find(&sessions)

	if result.Error != nil {
		return fmt.Errorf("failed to find old sessions: %w", result.Error)
	}

	s.logger.Info("Found old sessions for cleanup",
		"count", len(sessions),
		"cutoffDate", cutoffDate,
	)

	// This would actually clean up the session directories
	// For now, just log what would be cleaned up
	for _, session := range sessions {
		s.logger.Info("Would cleanup session",
			"sessionID", session.ID,
			"directoryPath", session.DirectoryPath,
			"contentHash", session.ContentHash,
		)
	}

	return nil
}

// CreateContentMigrationCallback returns a callback function for pipeline providers
func (s *ContentMigrationService) CreateContentMigrationCallback() func(sessionID, contentHash, contentURL string) {
	return func(sessionID, contentHash, contentURL string) {
		if err := s.UpdateSessionContentHash(sessionID, contentHash, contentURL); err != nil {
			logger.Error("Failed to update session content hash",
				"sessionID", sessionID,
				"contentHash", contentHash,
				"error", err,
			)
		} else {
			logger.Info("Successfully updated session with content hash",
				"sessionID", sessionID,
				"contentHash", contentHash,
			)
		}
	}
}
