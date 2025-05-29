package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockEventBus implements events.EventBus for testing
type MockEventBus struct {
	events []events.Event
	mu     sync.RWMutex
}

func (m *MockEventBus) Publish(ctx context.Context, event events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *MockEventBus) PublishAsync(event events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *MockEventBus) Subscribe(ctx context.Context, filter events.EventFilter, handler events.EventHandler) (*events.Subscription, error) {
	return nil, nil
}

func (m *MockEventBus) Unsubscribe(subscriptionID string) error {
	return nil
}

func (m *MockEventBus) GetSubscriptions() []*events.Subscription {
	return nil
}

func (m *MockEventBus) GetEvents(filter events.EventFilter, limit, offset int) ([]events.Event, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]events.Event{}, m.events...), int64(len(m.events)), nil
}

func (m *MockEventBus) GetEventsByTimeRange(start, end time.Time, limit, offset int) ([]events.Event, int64, error) {
	return m.GetEvents(events.EventFilter{}, limit, offset)
}

func (m *MockEventBus) GetStats() events.EventStats {
	return events.EventStats{}
}

func (m *MockEventBus) DeleteEvent(ctx context.Context, eventID string) error {
	return nil
}

func (m *MockEventBus) ClearEvents(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
	return nil
}

func (m *MockEventBus) Start(ctx context.Context) error {
	return nil
}

func (m *MockEventBus) Stop(ctx context.Context) error {
	return nil
}

func (m *MockEventBus) Health() error {
	return nil
}

func (m *MockEventBus) GetEventsForTest() []events.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]events.Event{}, m.events...)
}

func (m *MockEventBus) ClearEventsForTest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	// Generate a unique database name for each test to ensure isolation
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err, "Failed to open test database: %v", err)

	// Get the underlying sql.DB and set a busy timeout if possible
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Exec("PRAGMA busy_timeout = 5000;") // 5 seconds
	} else {
		t.Logf("Warning: Could not get underlying sql.DB to set busy_timeout: %v", err)
	}

	err = db.AutoMigrate(
		&database.MediaLibrary{},
		&database.ScanJob{},
		&database.MediaFile{},
		&database.MusicMetadata{},
	)
	require.NoError(t, err)
	return db
}

// createTestLibrary creates a test media library
func createTestLibrary(t *testing.T, db *gorm.DB, path string) *database.MediaLibrary {
	library := &database.MediaLibrary{
		Path: path,
		Type: "music",
	}
	err := db.Create(library).Error
	require.NoError(t, err)
	return library
}

// createTestDirectory creates a temporary directory with test files
func createTestDirectory(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "scanner_test_*")
	require.NoError(t, err)

	testFiles := []string{
		"song1.mp3",
		"song2.flac",
		"album1/track1.mp3",
		"album1/track2.mp3",
		"album2/song.wav",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("test audio data"), 0644)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})
	return tempDir
}

// Minimal test to ensure the file is created and compiles with utilities
func TestGetWorkerStats(t *testing.T) {
	db := setupTestDB(t)
	testDir := createTestDirectory(t)
	library := createTestLibrary(t, db, testDir)

	scanJob := &database.ScanJob{
		LibraryID: library.ID,
		Status:    "running",
	}
	err := db.Create(scanJob).Error
	require.NoError(t, err)

	eventBus := &MockEventBus{}
	scanner := NewLibraryScanner(db, scanJob.ID, eventBus, nil)

	active, min, max, queueLen := scanner.GetWorkerStats()

	assert.GreaterOrEqual(t, active, 0)
	assert.GreaterOrEqual(t, min, 0)
	assert.Greater(t, max, 0)
	assert.GreaterOrEqual(t, queueLen, 0)
	assert.LessOrEqual(t, min, max)
}
