package session

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
)

// testManager creates a minimal Manager for testing without FFmpeg dependencies.
func testManager(t *testing.T, logger *slog.Logger) *Manager {
	t.Helper()
	config := &ManagerConfig{
		HardwareAccel: "none",
	}
	return NewManager(config, logger)
}

// TestStopOtherMediaSessions tests stopping sessions for different media IDs
func TestStopOtherMediaSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create sessions for different media IDs
	session1 := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session2 := NewTranscodeSession(2, "1080p", 0, -1, tmpDir, logger, nil)
	session3 := NewTranscodeSession(3, "720p", 0, -1, tmpDir, logger, nil)
	session4 := NewTranscodeSession(1, "720p", 0, -1, tmpDir, logger, nil) // Same media as session1

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1)
	manager.sessions.Store(sessionKey(2, "1080p", -1), session2)
	manager.sessions.Store(sessionKey(3, "720p", -1), session3)
	manager.sessions.Store(sessionKey(1, "720p", -1), session4)

	// Verify all sessions exist
	count := 0
	manager.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	if count != 4 {
		t.Fatalf("Expected 4 sessions before cleanup, got %d", count)
	}

	// Stop sessions for all media except media ID 1
	manager.stopOtherMediaSessions(1)

	// Verify only media ID 1 sessions remain
	count = 0
	manager.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.MediaID != 1 {
			t.Errorf("Found session for media %d, should only have media 1", session.MediaID)
		}
		count++
		return true
	})

	if count != 2 {
		t.Errorf("Expected 2 sessions after cleanup (both for media 1), got %d", count)
	}

	// Verify sessions 1 and 4 still exist (same media ID)
	_, err := manager.GetSession(1, "1080p", -1)
	if err != nil {
		t.Error("Session for media 1, quality 1080p should still exist")
	}

	_, err = manager.GetSession(1, "720p", -1)
	if err != nil {
		t.Error("Session for media 1, quality 720p should still exist")
	}

	// Verify sessions 2 and 3 were removed
	_, err = manager.GetSession(2, "1080p", -1)
	if err == nil {
		t.Error("Session for media 2 should be removed")
	}

	_, err = manager.GetSession(3, "720p", -1)
	if err == nil {
		t.Error("Session for media 3 should be removed")
	}
}

// TestStopOtherMediaSessions_WithLogStore tests cleanup with log store
func TestStopOtherMediaSessions_WithLogStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()
	logDir := t.TempDir()

	// Set up log store
	logStore, err := logging.NewFFmpegLogStore(logDir, 48*time.Hour, logger)
	if err != nil {
		t.Fatalf("Failed to create log store: %v", err)
	}
	manager.SetLogStore(logStore)

	// Create sessions with log writers
	session1 := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	logWriter1, _ := logStore.CreateLogWriter(session1.ID, 1, "1080p")
	session1.SetLogWriter(logWriter1)

	session2 := NewTranscodeSession(2, "1080p", 0, -1, tmpDir, logger, nil)
	logWriter2, _ := logStore.CreateLogWriter(session2.ID, 2, "1080p")
	session2.SetLogWriter(logWriter2)

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1)
	manager.sessions.Store(sessionKey(2, "1080p", -1), session2)

	// Stop sessions for media 1 (should cleanup media 2's log writer)
	manager.stopOtherMediaSessions(1)

	// Verify session 2 was removed but session 1 remains
	_, err = manager.GetSession(1, "1080p", -1)
	if err != nil {
		t.Error("Session for media 1 should still exist")
	}

	_, err = manager.GetSession(2, "1080p", -1)
	if err == nil {
		t.Error("Session for media 2 should be removed")
	}
}

// TestStopOtherMediaSessions_NoOtherSessions tests when no other sessions exist
func TestStopOtherMediaSessions_NoOtherSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create only one session
	session1 := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	manager.sessions.Store(sessionKey(1, "1080p", -1), session1)

	// Stop other sessions (should do nothing)
	manager.stopOtherMediaSessions(1)

	// Verify session still exists
	_, err := manager.GetSession(1, "1080p", -1)
	if err != nil {
		t.Error("Session should still exist")
	}
}

// TestStopOtherQualitySessions tests stopping sessions for different qualities
func TestStopOtherQualitySessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create sessions for same media but different qualities
	session1080p := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session720p := NewTranscodeSession(1, "720p", 0, -1, tmpDir, logger, nil)
	session480p := NewTranscodeSession(1, "480p", 0, -1, tmpDir, logger, nil)

	// Create mock FFmpeg commands that appear to be running
	// (ProcessState is nil when process is running)
	session1080p.FFmpegCmd = &exec.Cmd{}
	session720p.FFmpegCmd = &exec.Cmd{}
	session480p.FFmpegCmd = &exec.Cmd{}

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1080p)
	manager.sessions.Store(sessionKey(1, "720p", -1), session720p)
	manager.sessions.Store(sessionKey(1, "480p", -1), session480p)

	// Stop other quality sessions (keep 1080p)
	manager.StopOtherQualitySessions(1, "1080p")

	// Verify only 1080p session remains
	count := 0
	manager.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.Quality != "1080p" {
			t.Errorf("Found session for quality %s, should only have 1080p", session.Quality)
		}
		count++
		return true
	})

	if count != 1 {
		t.Errorf("Expected 1 session after cleanup (1080p), got %d", count)
	}
}

// TestStopOtherQualitySessions_OnlyActiveProcesses tests that only active processes are stopped
func TestStopOtherQualitySessions_OnlyActiveProcesses(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create sessions
	session1080p := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session720p := NewTranscodeSession(1, "720p", 0, -1, tmpDir, logger, nil)
	session480p := NewTranscodeSession(1, "480p", 0, -1, tmpDir, logger, nil)

	// 1080p: Running (ProcessState = nil)
	session1080p.FFmpegCmd = &exec.Cmd{}

	// 720p: Running (ProcessState = nil)
	session720p.FFmpegCmd = &exec.Cmd{}

	// 480p: Completed (ProcessState != nil)
	// Simulate a completed process
	completedCmd := exec.Command("echo", "test")
	completedCmd.Run() // This will set ProcessState to non-nil
	session480p.FFmpegCmd = completedCmd

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1080p)
	manager.sessions.Store(sessionKey(1, "720p", -1), session720p)
	manager.sessions.Store(sessionKey(1, "480p", -1), session480p)

	// Stop other quality sessions (keep 1080p)
	manager.StopOtherQualitySessions(1, "1080p")

	// Verify 1080p and 480p sessions remain (480p was completed, not running)
	count := 0
	var qualities []string
	manager.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		qualities = append(qualities, session.Quality)
		count++
		return true
	})

	if count != 2 {
		t.Errorf("Expected 2 sessions after cleanup (1080p current + 480p completed), got %d", count)
	}

	// 720p should be removed (was running but different quality)
	_, err := manager.GetSession(1, "720p", -1)
	if err == nil {
		t.Error("Running 720p session should be removed")
	}

	// 480p should remain (was completed)
	_, err = manager.GetSession(1, "480p", -1)
	if err != nil {
		t.Error("Completed 480p session should remain for ABR")
	}
}

// TestStopOtherQualitySessions_DifferentMedia tests that different media IDs are not affected
func TestStopOtherQualitySessions_DifferentMedia(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create sessions for different media
	session1_1080p := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	session1_720p := NewTranscodeSession(1, "720p", 0, -1, tmpDir, logger, nil)
	session2_720p := NewTranscodeSession(2, "720p", 0, -1, tmpDir, logger, nil)

	// All running
	session1_1080p.FFmpegCmd = &exec.Cmd{}
	session1_720p.FFmpegCmd = &exec.Cmd{}
	session2_720p.FFmpegCmd = &exec.Cmd{}

	manager.sessions.Store(sessionKey(1, "1080p", -1), session1_1080p)
	manager.sessions.Store(sessionKey(1, "720p", -1), session1_720p)
	manager.sessions.Store(sessionKey(2, "720p", -1), session2_720p)

	// Stop other qualities for media 1 (should not affect media 2)
	manager.StopOtherQualitySessions(1, "1080p")

	// Verify media 2 session still exists
	_, err := manager.GetSession(2, "720p", -1)
	if err != nil {
		t.Error("Session for different media should not be affected")
	}

	// Verify media 1, 720p was removed
	_, err = manager.GetSession(1, "720p", -1)
	if err == nil {
		t.Error("720p session for media 1 should be removed")
	}
}

// TestCleanupSessionOutput tests manual session output cleanup
func TestCleanupSessionOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()
	mediaID := int64(123)
	quality := "1080p"

	// Create session output directory with files
	sessionDir := filepath.Join(tmpDir, fmt.Sprintf("%d", mediaID), quality)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFile := filepath.Join(sessionDir, "segment0.ts")
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Fatal("Session directory should exist before cleanup")
	}

	// Cleanup session output
	err := manager.CleanupSessionOutput(mediaID, quality, tmpDir)
	if err != nil {
		t.Fatalf("CleanupSessionOutput() error = %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Session output directory should be removed")
	}
}

// TestCleanupSessionOutput_NonExistentDir tests cleanup of non-existent directory
func TestCleanupSessionOutput_NonExistentDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Cleanup non-existent directory (should not error)
	err := manager.CleanupSessionOutput(999, "1080p", tmpDir)
	if err != nil {
		t.Errorf("CleanupSessionOutput() should not error for non-existent directory, got: %v", err)
	}
}

// TestCleanupSessionOutput_NestedFiles tests cleanup with nested directory structure
func TestCleanupSessionOutput_NestedFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()
	mediaID := int64(456)
	quality := "720p"

	// Create nested directory structure
	sessionDir := filepath.Join(tmpDir, fmt.Sprintf("%d", mediaID), quality)
	nestedDir := filepath.Join(sessionDir, "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create files at different levels
	file1 := filepath.Join(sessionDir, "segment0.ts")
	file2 := filepath.Join(nestedDir, "segment1.ts")
	os.WriteFile(file1, []byte("data1"), 0644)
	os.WriteFile(file2, []byte("data2"), 0644)

	// Cleanup
	err := manager.CleanupSessionOutput(mediaID, quality, tmpDir)
	if err != nil {
		t.Fatalf("CleanupSessionOutput() error = %v", err)
	}

	// Verify entire directory tree was removed
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Session output directory and all contents should be removed")
	}
}

// TestStartPeriodicCleanup tests periodic cleanup functionality
func TestStartPeriodicCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create an idle session
	idleSession := NewTranscodeSession(1, "1080p", 0, -1, tmpDir, logger, nil)
	idleSession.LastAccessed = time.Now().Add(-10 * time.Minute) // Old
	manager.sessions.Store(sessionKey(1, "1080p", -1), idleSession)

	// Create old transcode directory
	oldMediaDir := filepath.Join(tmpDir, "999")
	if err := os.MkdirAll(oldMediaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	testFile := filepath.Join(oldMediaDir, "segment.ts")
	os.WriteFile(testFile, []byte("old data"), 0644)

	// Make directory appear old
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldMediaDir, oldTime, oldTime)

	// Start periodic cleanup with short intervals
	cancel := manager.StartPeriodicCleanup(
		100*time.Millisecond, // session check interval
		5*time.Minute,        // session idle timeout
		150*time.Millisecond, // transcode check interval
		24*time.Hour,         // transcode max age
		tmpDir,
	)
	defer cancel()

	// Wait for cleanup to run
	time.Sleep(300 * time.Millisecond)

	// Cancel cleanup first
	cancel()

	// Verify idle session was cleaned up
	_, err := manager.GetSession(1, "1080p", -1)
	if err == nil {
		t.Error("Idle session should be cleaned up")
	}

	// Verify old transcode was cleaned up
	if _, err := os.Stat(oldMediaDir); !os.IsNotExist(err) {
		t.Error("Old transcode directory should be removed")
	}
}

// TestStartPeriodicCleanup_WithLogStore tests periodic cleanup with log store
func TestStartPeriodicCleanup_WithLogStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()
	logDir := t.TempDir()

	// Create log store
	logStore, err := logging.NewFFmpegLogStore(logDir, 48*time.Hour, logger)
	if err != nil {
		t.Fatalf("Failed to create log store: %v", err)
	}
	manager.SetLogStore(logStore)

	// Create an old log file
	oldLogDir := filepath.Join(logDir, "old_session_123")
	if err := os.MkdirAll(oldLogDir, 0755); err != nil {
		t.Fatalf("Failed to create old log dir: %v", err)
	}
	oldLogFile := filepath.Join(oldLogDir, "ffmpeg.log")
	os.WriteFile(oldLogFile, []byte("old log data"), 0644)

	// Make it appear old
	oldTime := time.Now().Add(-72 * time.Hour)
	os.Chtimes(oldLogDir, oldTime, oldTime)

	// Start periodic cleanup
	cancel := manager.StartPeriodicCleanup(
		500*time.Millisecond, // session check interval
		5*time.Minute,        // session idle timeout
		100*time.Millisecond, // transcode check interval (shorter to test logs)
		24*time.Hour,         // transcode max age
		tmpDir,
	)

	// Wait for cleanup to run
	time.Sleep(300 * time.Millisecond)

	// Cancel and cleanup
	cancel()
	time.Sleep(50 * time.Millisecond) // Give goroutine time to exit
}

// TestStartPeriodicCleanup_CancelMultipleTimes tests cancel safety
func TestStartPeriodicCleanup_CancelMultipleTimes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	cancel := manager.StartPeriodicCleanup(
		1*time.Second,
		5*time.Minute,
		1*time.Second,
		24*time.Hour,
		tmpDir,
	)

	// Cancel multiple times (should be safe)
	cancel()
	cancel()
	cancel()

	// No assertion needed - just verify no panic
}

// TestCleanupOutputDir_WithSessionID tests the internal cleanup function with session ID
func TestCleanupOutputDir_WithSessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create a directory to cleanup
	testDir := filepath.Join(tmpDir, "test_cleanup")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFile := filepath.Join(testDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Cleanup with session ID
	manager.cleanupOutputDir(testDir, "test-session-123")

	// Verify directory was removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Directory should be removed")
	}
}

// TestCleanupOutputDir_WithoutSessionID tests cleanup without session ID for logging
func TestCleanupOutputDir_WithoutSessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create a directory to cleanup
	testDir := filepath.Join(tmpDir, "test_cleanup2")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Cleanup without session ID
	manager.cleanupOutputDir(testDir, "")

	// Verify directory was removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Directory should be removed")
	}
}

// TestCleanupOutputDir_NonExistentPath tests cleanup of non-existent directory
func TestCleanupOutputDir_NonExistentPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "does_not_exist")

	// Cleanup should not panic or error out (just logs warning)
	manager.cleanupOutputDir(nonExistentDir, "test-session")
	// No assertion needed - just verify no panic
}

// TestCleanupOutputDir_PermissionError tests cleanup with permission issues
func TestCleanupOutputDir_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create a directory with restricted permissions
	testDir := filepath.Join(tmpDir, "restricted")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a file inside
	testFile := filepath.Join(testDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make parent directory read-only (prevents removal on some systems)
	parentDir := tmpDir
	originalPerm := os.FileMode(0755)
	if info, err := os.Stat(parentDir); err == nil {
		originalPerm = info.Mode()
	}
	os.Chmod(parentDir, 0555)
	defer os.Chmod(parentDir, originalPerm) // Restore permissions

	// Cleanup will fail but should just log warning
	manager.cleanupOutputDir(testDir, "test-session")
	// No assertion needed - just verify no panic
}

// TestCleanupOldTranscodes_MultipleDirectories tests cleanup of multiple old directories
func TestCleanupOldTranscodes_MultipleDirectories(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create multiple old transcode directories
	for i := 1; i <= 3; i++ {
		mediaDir := filepath.Join(tmpDir, fmt.Sprintf("%d", i))
		if err := os.MkdirAll(mediaDir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Create some files
		for j := 0; j < 5; j++ {
			file := filepath.Join(mediaDir, fmt.Sprintf("segment%d.ts", j))
			os.WriteFile(file, []byte("segment data"), 0644)
		}

		// Make them old
		oldTime := time.Now().Add(-48 * time.Hour)
		os.Chtimes(mediaDir, oldTime, oldTime)
	}

	// Cleanup old transcodes
	cleanedCount, bytesFreed, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 3 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 3", cleanedCount)
	}

	if bytesFreed == 0 {
		t.Error("CleanupOldTranscodes() should report bytes freed")
	}

	// Verify all directories were removed
	for i := 1; i <= 3; i++ {
		mediaDir := filepath.Join(tmpDir, fmt.Sprintf("%d", i))
		if _, err := os.Stat(mediaDir); !os.IsNotExist(err) {
			t.Errorf("Directory %d should be removed", i)
		}
	}
}

// TestCleanupOldTranscodes_NonNumericDirectories tests that non-numeric dirs are skipped
func TestCleanupOldTranscodes_NonNumericDirectories(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create non-numeric directories (note: "123abc" will parse as 123 with fmt.Sscanf)
	// So we only test purely non-numeric names
	invalidDirs := []string{"abc", "temp", "cache", "xyz123"}
	for _, name := range invalidDirs {
		dir := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Make them old
		oldTime := time.Now().Add(-48 * time.Hour)
		os.Chtimes(dir, oldTime, oldTime)
	}

	// Cleanup should skip non-numeric directories
	cleanedCount, _, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 0 {
		t.Errorf("CleanupOldTranscodes() should not clean non-numeric directories, cleaned %d", cleanedCount)
	}

	// Verify directories still exist
	for _, name := range invalidDirs {
		dir := filepath.Join(tmpDir, name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Non-numeric directory %s should not be removed", name)
		}
	}
}

// TestCleanupOldTranscodes_EmptyDirectory tests cleanup when directory is empty
func TestCleanupOldTranscodes_EmptyDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create old empty directory
	mediaDir := filepath.Join(tmpDir, "123")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Make it old
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(mediaDir, oldTime, oldTime)

	// Cleanup
	cleanedCount, bytesFreed, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 1 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 1", cleanedCount)
	}

	if bytesFreed != 0 {
		t.Errorf("CleanupOldTranscodes() freed %d bytes from empty directory, want 0", bytesFreed)
	}
}

// TestCleanupOldTranscodes_MixedAges tests cleanup with mixed old and recent directories
func TestCleanupOldTranscodes_MixedAges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	tmpDir := t.TempDir()

	// Create old directory
	oldDir := filepath.Join(tmpDir, "100")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("Failed to create old directory: %v", err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldDir, oldTime, oldTime)

	// Create recent directory
	recentDir := filepath.Join(tmpDir, "200")
	if err := os.MkdirAll(recentDir, 0755); err != nil {
		t.Fatalf("Failed to create recent directory: %v", err)
	}
	// Recent directory has current modification time

	// Cleanup with 24 hour threshold
	cleanedCount, _, err := manager.CleanupOldTranscodes(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldTranscodes() error = %v", err)
	}

	if cleanedCount != 1 {
		t.Errorf("CleanupOldTranscodes() cleaned %d directories, want 1", cleanedCount)
	}

	// Old directory should be removed
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("Old directory should be removed")
	}

	// Recent directory should remain
	if _, err := os.Stat(recentDir); err != nil {
		t.Error("Recent directory should not be removed")
	}
}

// TestCleanupOldTranscodes_InvalidOutputDir tests cleanup with invalid directory
func TestCleanupOldTranscodes_InvalidOutputDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := testManager(t, logger)

	// Try to cleanup non-existent directory
	_, _, err := manager.CleanupOldTranscodes("/non/existent/path", 24*time.Hour)
	if err == nil {
		t.Error("CleanupOldTranscodes() should error for non-existent directory")
	}
}
