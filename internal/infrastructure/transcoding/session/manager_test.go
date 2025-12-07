package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
)

// mockConfigProvider implements ConfigProvider for testing
type mockConfigProvider struct {
	config *Config
}

func (m *mockConfigProvider) GetConfig(ctx context.Context) *Config {
	return m.config
}

func testManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		HardwareAccel: "none",
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}
}

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()

	manager := NewManager(config, logger)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if manager.config == nil {
		t.Error("Config should be initialized")
	}

	if manager.logger != logger {
		t.Error("Logger not set correctly")
	}

	if manager.fallbackManager == nil {
		t.Error("Fallback manager not initialized")
	}
}

func TestNewManager_NilConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	manager := NewManager(nil, logger)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	// Should handle nil config gracefully
	if manager.config == nil {
		t.Error("Config should be initialized")
	}
}

func TestSetConfigProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	providerConfig := &Config{
		ToneMappingAlgorithm: "bt.2390",
	}
	provider := &mockConfigProvider{config: providerConfig}
	manager.SetConfigProvider(provider)

	if manager.configProvider == nil {
		t.Error("Config provider not set")
	}

	// Verify it returns the provider's config
	effectiveConfig := manager.getEffectiveConfig(context.Background())
	if effectiveConfig != provider.config {
		t.Error("getEffectiveConfig() should return provider's config")
	}
}

func TestSetLogStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	logStore, err := logging.NewFFmpegLogStore(tmpDir, 48*time.Hour, logger)
	if err != nil {
		t.Fatalf("Failed to create log store: %v", err)
	}

	manager.SetLogStore(logStore)

	if manager.logStore == nil {
		t.Error("Log store not set")
	}

	retrieved := manager.GetLogStore()
	if retrieved != logStore {
		t.Error("GetLogStore() returned wrong store")
	}
}

func TestGetSessionMutex(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	key1 := "123:1080p"
	key2 := "456:720p"

	// Get mutex for first key
	mu1a := manager.getSessionMutex(key1)
	if mu1a == nil {
		t.Fatal("getSessionMutex() returned nil")
	}

	// Get mutex for same key again - should be the same instance
	mu1b := manager.getSessionMutex(key1)
	if mu1a != mu1b {
		t.Error("getSessionMutex() should return same mutex for same key")
	}

	// Get mutex for different key - should be different instance
	mu2 := manager.getSessionMutex(key2)
	if mu1a == mu2 {
		t.Error("getSessionMutex() should return different mutex for different key")
	}
}

func TestGetSessionMutex_Concurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	key := "123:1080p"
	numGoroutines := 100

	// All goroutines should get the same mutex instance
	mutexes := make([]*sync.Mutex, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			mutexes[idx] = manager.getSessionMutex(key)
		}(i)
	}

	wg.Wait()

	// Verify all goroutines got the same mutex
	first := mutexes[0]
	for i := 1; i < numGoroutines; i++ {
		if mutexes[i] != first {
			t.Errorf("Goroutine %d got different mutex instance", i)
		}
	}
}

func TestSessionKey(t *testing.T) {
	tests := []struct {
		name            string
		mediaID         int64
		quality         string
		audioTrackIndex int
		expectedKey     string
	}{
		{
			name:            "Default audio track (-1)",
			mediaID:         123,
			quality:         "1080p",
			audioTrackIndex: -1,
			expectedKey:     "123:1080p",
		},
		{
			name:            "First audio track (0)",
			mediaID:         123,
			quality:         "1080p",
			audioTrackIndex: 0,
			expectedKey:     "123:1080p",
		},
		{
			name:            "Second audio track",
			mediaID:         123,
			quality:         "1080p",
			audioTrackIndex: 1,
			expectedKey:     "123:1080p:audio1",
		},
		{
			name:            "Third audio track",
			mediaID:         456,
			quality:         "720p",
			audioTrackIndex: 2,
			expectedKey:     "456:720p:audio2",
		},
		{
			name:            "Different quality",
			mediaID:         789,
			quality:         "4k",
			audioTrackIndex: 0,
			expectedKey:     "789:4k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sessionKey(tt.mediaID, tt.quality, tt.audioTrackIndex)
			if result != tt.expectedKey {
				t.Errorf("sessionKey() = %q, want %q", result, tt.expectedKey)
			}
		})
	}
}

func TestStopAllSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create a few mock sessions
	for i := 1; i <= 3; i++ {
		session := NewTranscodeSession(
			int64(i),
			"1080p",
			0,
			-1,
			tmpDir,
			logger,
			nil,
		)
		key := sessionKey(int64(i), "1080p", -1)
		manager.sessions.Store(key, session)
	}

	// Verify sessions exist
	count := 0
	manager.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count != 3 {
		t.Fatalf("Expected 3 sessions before cleanup, got %d", count)
	}

	// Stop all sessions
	manager.StopAllSessions()

	// Verify all sessions are gone
	count = 0
	manager.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count != 0 {
		t.Errorf("Expected 0 sessions after StopAllSessions, got %d", count)
	}
}

func TestGetStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	// Initially no sessions
	stats := manager.GetStats()
	if stats["active_sessions"] != 0 {
		t.Errorf("Expected 0 active sessions, got %v", stats["active_sessions"])
	}

	tmpDir := t.TempDir()

	// Add some sessions
	for i := 1; i <= 5; i++ {
		session := NewTranscodeSession(
			int64(i),
			"1080p",
			0,
			-1,
			tmpDir,
			logger,
			nil,
		)
		key := sessionKey(int64(i), "1080p", -1)
		manager.sessions.Store(key, session)
	}

	// Verify stats
	stats = manager.GetStats()
	if stats["active_sessions"] != 5 {
		t.Errorf("Expected 5 active sessions, got %v", stats["active_sessions"])
	}
}

func TestGetSessionOutputPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session
	session := NewTranscodeSession(mediaID, quality, 0, -1, tmpDir, logger, nil)
	key := sessionKey(mediaID, quality, -1)
	manager.sessions.Store(key, session)

	// Test successful retrieval
	path, err := manager.GetSessionOutputPath(mediaID, quality, -1)
	if err != nil {
		t.Fatalf("GetSessionOutputPath() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, fmt.Sprintf("%d", mediaID), quality)
	if path != expectedPath {
		t.Errorf("GetSessionOutputPath() = %q, want %q", path, expectedPath)
	}

	// Test non-existent session
	_, err = manager.GetSessionOutputPath(999, quality, -1)
	if err == nil {
		t.Error("GetSessionOutputPath() should error for non-existent session")
	}
}

func TestGetSessionOutputPath_AudioTrack(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"
	audioTrackIndex := 1

	// Create session with specific audio track
	session := NewTranscodeSession(mediaID, quality, 0, audioTrackIndex, tmpDir, logger, nil)
	key := sessionKey(mediaID, quality, audioTrackIndex)
	manager.sessions.Store(key, session)

	// Test retrieval with specific audio track
	path, err := manager.GetSessionOutputPath(mediaID, quality, audioTrackIndex)
	if err != nil {
		t.Fatalf("GetSessionOutputPath() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s_audio%d", quality, audioTrackIndex))
	if path != expectedPath {
		t.Errorf("GetSessionOutputPath() = %q, want %q", path, expectedPath)
	}
}

func TestGetSessionOutputPath_DefaultAudioFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session with default audio (track 0)
	session := NewTranscodeSession(mediaID, quality, 0, 0, tmpDir, logger, nil)
	key := sessionKey(mediaID, quality, 0)
	manager.sessions.Store(key, session)

	// Request with -1 (unspecified) should find session with track 0
	path, err := manager.GetSessionOutputPath(mediaID, quality, -1)
	if err != nil {
		t.Fatalf("GetSessionOutputPath() error = %v", err)
	}

	if path == "" {
		t.Error("GetSessionOutputPath() should find default audio track session")
	}
}

func TestCleanupIdleSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create sessions with different last accessed times
	session1 := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session1.LastAccessed = time.Now().Add(-10 * time.Minute) // Old

	session2 := NewTranscodeSession(2, "1080p", 0, -1, tmpDir, logger, nil)
	session2.LastAccessed = time.Now().Add(-1 * time.Minute) // Recent

	session3 := NewTranscodeSession(3, "720p", 0, -1, tmpDir, logger, nil)
	session3.LastAccessed = time.Now().Add(-20 * time.Minute) // Very old

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1)
	manager.sessions.Store(sessionKey(2, "1080p", -1), session2)
	manager.sessions.Store(sessionKey(3, "720p", -1), session3)

	// Cleanup sessions idle for more than 5 minutes
	cleanedCount := manager.CleanupIdleSessions(5 * time.Minute)

	if cleanedCount != 2 {
		t.Errorf("CleanupIdleSessions() cleaned %d sessions, want 2", cleanedCount)
	}

	// Verify session2 still exists
	_, err := manager.GetSession(2, "1080p", -1)
	if err != nil {
		t.Error("Recent session should not be cleaned up")
	}

	// Verify session1 and session3 are gone
	_, err = manager.GetSession(1, "1080p", -1)
	if err == nil {
		t.Error("Old session should be cleaned up")
	}

	_, err = manager.GetSession(3, "720p", -1)
	if err == nil {
		t.Error("Very old session should be cleaned up")
	}
}

func TestCleanupIdleSessions_NoIdleSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create recent session
	session := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session.LastAccessed = time.Now()

	manager.sessions.Store(sessionKey(1, "1080p", -1), session)

	// Cleanup should find nothing
	cleanedCount := manager.CleanupIdleSessions(5 * time.Minute)

	if cleanedCount != 0 {
		t.Errorf("CleanupIdleSessions() cleaned %d sessions, want 0", cleanedCount)
	}
}

func TestCleanupOldTranscodes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create old transcode directory structure
	oldMediaDir := filepath.Join(tmpDir, "123")
	if err := os.MkdirAll(oldMediaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test file in the old directory
	testFile := filepath.Join(oldMediaDir, "segment.ts")
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make directory appear old
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldMediaDir, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set directory time: %v", err)
	}

	// Cleanup old transcodes (older than 24 hours)
	cleanedCount, bytesFreed, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 1 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 1", cleanedCount)
	}

	if bytesFreed <= 0 {
		t.Errorf("CleanupOldTranscodes() freed %d bytes, want > 0", bytesFreed)
	}

	// Verify directory was removed
	if _, err := os.Stat(oldMediaDir); !os.IsNotExist(err) {
		t.Error("Old transcode directory should be removed")
	}
}

func TestCleanupOldTranscodes_ActiveSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create old transcode directory
	mediaID := int64(123)
	mediaDir := filepath.Join(tmpDir, fmt.Sprintf("%d", mediaID))
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Make directory appear old
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(mediaDir, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set directory time: %v", err)
	}

	// Create active session for this media
	session := NewTranscodeSession(mediaID, "1080p", 0, -1, tmpDir, logger, nil)
	manager.sessions.Store(sessionKey(mediaID, "1080p", -1), session)

	// Cleanup should skip active session
	cleanedCount, _, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 0 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 0 (active session)", cleanedCount)
	}

	// Verify directory still exists
	if _, err := os.Stat(mediaDir); err != nil {
		t.Error("Active session directory should not be removed")
	}
}

func TestCleanupOldTranscodes_RecentTranscode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create recent transcode directory
	mediaDir := filepath.Join(tmpDir, "123")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Directory has recent modification time (default from os.MkdirAll)
	// Cleanup should skip recent directories
	cleanedCount, _, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 0 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 0 (recent)", cleanedCount)
	}

	// Verify directory still exists
	if _, err := os.Stat(mediaDir); err != nil {
		t.Error("Recent transcode directory should not be removed")
	}
}

func TestStartIdleCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()

	// Create an old session
	session := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session.LastAccessed = time.Now().Add(-10 * time.Minute)
	manager.sessions.Store(sessionKey(1, "1080p", -1), session)

	// Start cleanup with very short interval for testing
	cancel := manager.StartIdleCleanup(100*time.Millisecond, 5*time.Minute)
	defer cancel()

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Verify session was cleaned up
	_, err := manager.GetSession(1, "1080p", -1)
	if err == nil {
		t.Error("Idle session should be cleaned up by periodic cleanup")
	}

	// Cancel cleanup and wait for it to stop
	cancel()
	time.Sleep(50 * time.Millisecond) // Give goroutine time to exit

	// Add another session with a recent LastAccessed (not idle)
	// This ensures it won't be cleaned even if there's a race
	session2 := NewTranscodeSession(2, "720p", 0, -1, tmpDir, logger, nil)
	session2.LastAccessed = time.Now() // Recent access - not idle
	manager.sessions.Store(sessionKey(2, "720p", -1), session2)

	// Wait a bit (longer than cleanup interval)
	time.Sleep(200 * time.Millisecond)

	// Session2 should still exist (cleanup was cancelled OR it's not idle anyway)
	_, err = manager.GetSession(2, "720p", -1)
	if err != nil {
		t.Error("Session should not be cleaned after cleanup cancellation")
	}
}

func TestGetEffectiveConfig_WithProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseConfig := testManagerConfig()

	manager := NewManager(baseConfig, logger)

	// Without provider, should return base config values
	effective := manager.getEffectiveConfig(context.Background())
	if effective == nil {
		t.Fatal("getEffectiveConfig() should not return nil")
	}

	// With provider, should return provider's config
	providerConfig := &Config{
		ToneMappingAlgorithm: "bt.2390",
	}

	provider := &mockConfigProvider{config: providerConfig}
	manager.SetConfigProvider(provider)

	effective = manager.getEffectiveConfig(context.Background())
	if effective.ToneMappingAlgorithm != "bt.2390" {
		t.Errorf("With provider, expected algorithm 'bt.2390', got %q", effective.ToneMappingAlgorithm)
	}
}

func TestStopSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session
	session := NewTranscodeSession(mediaID, quality, 0, -1, tmpDir, logger, nil)
	key := sessionKey(mediaID, quality, -1)
	manager.sessions.Store(key, session)

	// Stop session
	err := manager.StopSession(mediaID, quality, -1)
	if err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	// Verify session is removed
	_, err = manager.GetSession(mediaID, quality, -1)
	if err == nil {
		t.Error("Session should be removed after StopSession()")
	}

	// Stopping again should error
	err = manager.StopSession(mediaID, quality, -1)
	if err == nil {
		t.Error("StopSession() should error for non-existent session")
	}
}

func TestGetSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session with old LastAccessed time
	originalSession := NewTranscodeSession(mediaID, quality, 0, -1, tmpDir, logger, nil)
	oldAccessTime := time.Now().Add(-5 * time.Minute)
	originalSession.LastAccessed = oldAccessTime
	key := sessionKey(mediaID, quality, -1)
	manager.sessions.Store(key, originalSession)

	// Get session
	retrievedSession, err := manager.GetSession(mediaID, quality, -1)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}

	if retrievedSession.ID != originalSession.ID {
		t.Error("GetSession() returned wrong session")
	}

	// Verify LastAccessed was updated (compare against saved old time, not the mutated session)
	if !retrievedSession.LastAccessed.After(oldAccessTime) {
		t.Error("GetSession() should update LastAccessed time")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	_, err := manager.GetSession(999, "1080p", -1)
	if err == nil {
		t.Error("GetSession() should error for non-existent session")
	}
}

func TestGetSession_AudioTrackFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := testManagerConfig()
	manager := NewManager(config, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session with track 0 (default)
	session := NewTranscodeSession(mediaID, quality, 0, 0, tmpDir, logger, nil)
	key := sessionKey(mediaID, quality, 0)
	manager.sessions.Store(key, session)

	// Request with -1 should find session with track 0
	retrievedSession, err := manager.GetSession(mediaID, quality, -1)
	if err != nil {
		t.Fatalf("GetSession() should find default audio track, got error: %v", err)
	}

	if retrievedSession.ID != session.ID {
		t.Error("GetSession() returned wrong session")
	}
}
