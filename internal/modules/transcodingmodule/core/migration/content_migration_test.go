package migration

import (
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&database.TranscodeSession{})
	require.NoError(t, err)

	return db
}

func TestUpdateSessionContentHash(t *testing.T) {
	db := setupTestDB(t)
	service := NewContentMigrationService(db)

	// Create a test session
	session := &database.TranscodeSession{
		ID:           "test-session-123",
		Provider:     "ffmpeg_pipeline",
		Status:       database.TranscodeStatusCompleted,
		StartTime:    time.Now(),
		LastAccessed: time.Now(),
	}
	err := db.Create(session).Error
	require.NoError(t, err)

	// Update content hash
	contentHash := "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"
	contentURL := "/api/v1/content/a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890/manifest.mpd"

	err = service.UpdateSessionContentHash("test-session-123", contentHash, contentURL)
	assert.NoError(t, err)

	// Verify the update
	var updatedSession database.TranscodeSession
	err = db.First(&updatedSession, "id = ?", "test-session-123").Error
	require.NoError(t, err)

	assert.Equal(t, contentHash, updatedSession.ContentHash)
}

func TestGetSessionContentHash(t *testing.T) {
	db := setupTestDB(t)
	service := NewContentMigrationService(db)

	contentHash := "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"

	// Create a test session with content hash
	session := &database.TranscodeSession{
		ID:           "test-session-456",
		Provider:     "ffmpeg_pipeline",
		Status:       database.TranscodeStatusCompleted,
		ContentHash:  contentHash,
		StartTime:    time.Now(),
		LastAccessed: time.Now(),
	}
	err := db.Create(session).Error
	require.NoError(t, err)

	// Get content hash
	retrievedHash, err := service.GetSessionContentHash("test-session-456")
	assert.NoError(t, err)
	assert.Equal(t, contentHash, retrievedHash)

	// Test non-existent session
	_, err = service.GetSessionContentHash("non-existent")
	assert.Error(t, err)
}

func TestListSessionsWithoutContentHash(t *testing.T) {
	db := setupTestDB(t)
	service := NewContentMigrationService(db)

	// Create sessions with different states
	sessions := []database.TranscodeSession{
		{
			ID:          "completed-with-hash",
			Status:      database.TranscodeStatusCompleted,
			ContentHash: "somehash",
			StartTime:   time.Now(),
		},
		{
			ID:        "completed-without-hash-1",
			Status:    database.TranscodeStatusCompleted,
			StartTime: time.Now(),
		},
		{
			ID:        "completed-without-hash-2",
			Status:    database.TranscodeStatusCompleted,
			StartTime: time.Now(),
		},
		{
			ID:        "running-without-hash",
			Status:    database.TranscodeStatusRunning,
			StartTime: time.Now(),
		},
	}

	for _, s := range sessions {
		s.LastAccessed = time.Now()
		err := db.Create(&s).Error
		require.NoError(t, err)
	}

	// List sessions without content hash
	result, err := service.ListSessionsWithoutContentHash(10)
	assert.NoError(t, err)
	assert.Len(t, result, 2) // Only completed sessions without hash

	// Verify the correct sessions were returned
	sessionIDs := make([]string, len(result))
	for i, s := range result {
		sessionIDs[i] = s.ID
	}
	assert.Contains(t, sessionIDs, "completed-without-hash-1")
	assert.Contains(t, sessionIDs, "completed-without-hash-2")
}

func TestGetContentHashStats(t *testing.T) {
	db := setupTestDB(t)
	service := NewContentMigrationService(db)

	// Create sessions with different states
	sessions := []database.TranscodeSession{
		{ID: "1", Status: database.TranscodeStatusCompleted, ContentHash: "hash1", StartTime: time.Now()},
		{ID: "2", Status: database.TranscodeStatusCompleted, ContentHash: "hash2", StartTime: time.Now()},
		{ID: "3", Status: database.TranscodeStatusCompleted, StartTime: time.Now()},
		{ID: "4", Status: database.TranscodeStatusFailed, StartTime: time.Now()},
		{ID: "5", Status: database.TranscodeStatusRunning, ContentHash: "hash5", StartTime: time.Now()},
	}

	for _, s := range sessions {
		s.LastAccessed = time.Now()
		err := db.Create(&s).Error
		require.NoError(t, err)
	}

	// Get stats
	stats, err := service.GetContentHashStats()
	assert.NoError(t, err)

	assert.Equal(t, int64(5), stats.TotalSessions)
	assert.Equal(t, int64(3), stats.SessionsWithHash)
	assert.Equal(t, int64(1), stats.CompletedWithoutHash)
	assert.Equal(t, float64(60), stats.HashCoveragePercent) // 3/5 = 60%
}

func TestContentMigrationCallback(t *testing.T) {
	db := setupTestDB(t)
	service := NewContentMigrationService(db)

	// Create a test session
	session := &database.TranscodeSession{
		ID:           "callback-test",
		Provider:     "ffmpeg_pipeline",
		Status:       database.TranscodeStatusRunning,
		StartTime:    time.Now(),
		LastAccessed: time.Now(),
	}
	err := db.Create(session).Error
	require.NoError(t, err)

	// Create and execute callback
	callback := service.CreateContentMigrationCallback()
	contentHash := "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"
	contentURL := "/api/v1/content/a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890/manifest.mpd"

	// Execute callback (it runs asynchronously in the actual implementation)
	callback("callback-test", contentHash, contentURL)

	// Give it a moment since the actual callback is async
	time.Sleep(100 * time.Millisecond)

	// Verify the update
	var updatedSession database.TranscodeSession
	err = db.First(&updatedSession, "id = ?", "callback-test").Error
	require.NoError(t, err)

	assert.Equal(t, contentHash, updatedSession.ContentHash)
}
