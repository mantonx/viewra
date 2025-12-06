package transcoding

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWaitForSegment_SegmentExists verifies WaitForSegment returns immediately
// when the segment is already marked as generated.
func TestWaitForSegment_SegmentExists(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Pre-mark segment 5 as generated
	session.segmentMutex.Lock()
	session.generatedSegments[5] = true
	session.segmentMutex.Unlock()

	// Should return immediately without timeout
	start := time.Now()
	segmentPath, err := session.WaitForSegment(5, 1*time.Second)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedPath := filepath.Join(session.OutputDir, SegmentFilename(5))
	if segmentPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, segmentPath)
	}

	// Should have returned very quickly (well under the timeout)
	if duration > 100*time.Millisecond {
		t.Errorf("Expected fast return, took %v", duration)
	}
}

// TestWaitForSegment_WaitsForSegment verifies WaitForSegment waits and returns
// when a segment appears after the wait begins.
func TestWaitForSegment_WaitsForSegment(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Start waiting in a goroutine
	resultChan := make(chan struct {
		path string
		err  error
	}, 1)

	go func() {
		path, err := session.WaitForSegment(10, 2*time.Second)
		resultChan <- struct {
			path string
			err  error
		}{path, err}
	}()

	// Wait a bit to ensure the goroutine is waiting
	time.Sleep(100 * time.Millisecond)

	// Now mark the segment as generated (simulating segment creation)
	session.segmentMutex.Lock()
	session.generatedSegments[10] = true
	session.segmentCond.Broadcast()
	session.segmentMutex.Unlock()

	// Should receive result quickly after broadcast
	select {
	case result := <-resultChan:
		if result.err != nil {
			t.Fatalf("Expected no error, got: %v", result.err)
		}
		expectedPath := filepath.Join(session.OutputDir, SegmentFilename(10))
		if result.path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, result.path)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForSegment did not return after segment was marked as generated")
	}
}

// TestWaitForSegment_Timeout verifies WaitForSegment times out correctly
// when the segment never appears.
func TestWaitForSegment_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	start := time.Now()
	_, err := session.WaitForSegment(99, 200*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should timeout close to the specified duration
	if duration < 200*time.Millisecond {
		t.Errorf("Timeout occurred too early: %v", duration)
	}
	if duration > 400*time.Millisecond {
		t.Errorf("Timeout took too long: %v", duration)
	}
}

// TestWaitForSegment_MultipleWaiters verifies that Broadcast wakes up
// multiple goroutines waiting for different segments.
func TestWaitForSegment_MultipleWaiters(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Start multiple waiters
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		wg.Add(1)
		segNum := 20 + i
		go func(seg int) {
			defer wg.Done()
			_, err := session.WaitForSegment(seg, 2*time.Second)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(segNum)
	}

	// Wait for goroutines to start waiting
	time.Sleep(100 * time.Millisecond)

	// Mark all segments as generated and broadcast
	session.segmentMutex.Lock()
	session.generatedSegments[20] = true
	session.generatedSegments[21] = true
	session.generatedSegments[22] = true
	session.segmentCond.Broadcast()
	session.segmentMutex.Unlock()

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mu.Lock()
		if successCount != 3 {
			t.Errorf("Expected 3 successful waits, got %d", successCount)
		}
		mu.Unlock()
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for all goroutines to complete")
	}
}

// TestWaitForManifest_ManifestExists verifies WaitForManifest returns immediately
// when both manifest and first segment exist.
func TestWaitForManifest_ManifestExists(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Set FFmpegStartedAt to past to avoid timestamp checks
	session.FFmpegStartedAt = time.Now().Add(-1 * time.Second)

	// Create manifest file
	manifestContent := []byte("#EXTM3U\n#EXT-X-VERSION:3\n")
	if err := os.WriteFile(session.ManifestPath, manifestContent, 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Create first segment (segment 0)
	firstSegmentPath := filepath.Join(session.OutputDir, SegmentFilename(0))
	if err := os.WriteFile(firstSegmentPath, []byte("fake segment data"), 0644); err != nil {
		t.Fatalf("Failed to create first segment: %v", err)
	}

	// Should return immediately
	start := time.Now()
	err := session.WaitForManifest(1 * time.Second)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have returned very quickly
	if duration > 200*time.Millisecond {
		t.Errorf("Expected fast return, took %v", duration)
	}

	// Verify ManifestReadyAt was set
	if session.ManifestReadyAt.IsZero() {
		t.Error("Expected ManifestReadyAt to be set")
	}
}

// TestWaitForManifest_WaitsForCreation verifies WaitForManifest waits
// for both manifest and first segment to be created.
func TestWaitForManifest_WaitsForCreation(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Set FFmpegStartedAt to now
	session.FFmpegStartedAt = time.Now()

	// Start waiting in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.WaitForManifest(2 * time.Second)
	}()

	// Wait a bit to ensure the goroutine is waiting
	time.Sleep(100 * time.Millisecond)

	// Create manifest file
	manifestContent := []byte("#EXTM3U\n#EXT-X-VERSION:3\n")
	if err := os.WriteFile(session.ManifestPath, manifestContent, 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Wait a bit more (still should be waiting for segment)
	time.Sleep(100 * time.Millisecond)

	// Create first segment
	firstSegmentPath := filepath.Join(session.OutputDir, SegmentFilename(0))
	if err := os.WriteFile(firstSegmentPath, []byte("fake segment data"), 0644); err != nil {
		t.Fatalf("Failed to create first segment: %v", err)
	}

	// Should receive result quickly after segment creation
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if session.ManifestReadyAt.IsZero() {
			t.Error("Expected ManifestReadyAt to be set")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForManifest did not return after files were created")
	}
}

// TestWaitForManifest_Timeout verifies WaitForManifest times out correctly
// when the manifest or segment never appears.
func TestWaitForManifest_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory but no files
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	session.FFmpegStartedAt = time.Now()

	start := time.Now()
	err := session.WaitForManifest(200 * time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should timeout close to the specified duration
	if duration < 200*time.Millisecond {
		t.Errorf("Timeout occurred too early: %v", duration)
	}
	if duration > 400*time.Millisecond {
		t.Errorf("Timeout took too long: %v", duration)
	}
}

// TestWaitForManifest_StartPosition verifies WaitForManifest waits for
// the correct first segment when StartPosition is set.
func TestWaitForManifest_StartPosition(t *testing.T) {
	tests := []struct {
		name            string
		startPosition   float64
		segmentDuration int
		expectedSegment int
	}{
		{
			name:            "start at 0 seconds",
			startPosition:   0,
			segmentDuration: 2,
			expectedSegment: 0,
		},
		{
			name:            "start at 10 seconds",
			startPosition:   10,
			segmentDuration: 2,
			expectedSegment: 5, // 10 / 2 = 5
		},
		{
			name:            "start at 37 seconds",
			startPosition:   37,
			segmentDuration: 2,
			expectedSegment: 18, // 37 / 2 = 18
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			session := createTestSession(tempDir, logger)
			session.StartPosition = tt.startPosition
			session.SegmentDurationSec = tt.segmentDuration
			defer session.Stop()

			// Create output directory
			if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
				t.Fatalf("Failed to create output dir: %v", err)
			}

			session.FFmpegStartedAt = time.Now().Add(-1 * time.Second)

			// Create manifest
			if err := os.WriteFile(session.ManifestPath, []byte("#EXTM3U\n"), 0644); err != nil {
				t.Fatalf("Failed to create manifest: %v", err)
			}

			// Create the WRONG segment (should not satisfy the wait)
			wrongSegmentPath := filepath.Join(session.OutputDir, SegmentFilename(0))
			if err := os.WriteFile(wrongSegmentPath, []byte("data"), 0644); err != nil {
				t.Fatalf("Failed to create wrong segment: %v", err)
			}

			// Start waiting in goroutine
			errChan := make(chan error, 1)
			go func() {
				errChan <- session.WaitForManifest(2 * time.Second)
			}()

			// Wait a bit - should still be waiting
			time.Sleep(100 * time.Millisecond)

			// Create the CORRECT segment
			correctSegmentPath := filepath.Join(session.OutputDir, SegmentFilename(tt.expectedSegment))
			if err := os.WriteFile(correctSegmentPath, []byte("data"), 0644); err != nil {
				t.Fatalf("Failed to create correct segment: %v", err)
			}

			// Should now complete
			select {
			case err := <-errChan:
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("WaitForManifest did not return after correct segment was created")
			}
		})
	}
}

// TestWaitForManifest_StaleManifestRejected verifies WaitForManifest rejects
// manifests created before FFmpeg started (stale from previous session).
func TestWaitForManifest_StaleManifestRejected(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create "stale" manifest from before FFmpeg started
	manifestContent := []byte("#EXTM3U\n#EXT-X-VERSION:3\n")
	if err := os.WriteFile(session.ManifestPath, manifestContent, 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Create stale first segment
	firstSegmentPath := filepath.Join(session.OutputDir, SegmentFilename(0))
	if err := os.WriteFile(firstSegmentPath, []byte("fake segment data"), 0644); err != nil {
		t.Fatalf("Failed to create first segment: %v", err)
	}

	// NOW set FFmpegStartedAt to AFTER the files were created
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
	session.FFmpegStartedAt = time.Now()

	// Start waiting - should timeout because files are too old
	start := time.Now()
	err := session.WaitForManifest(300 * time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error for stale manifest, got nil")
	}

	// Should have timed out
	if duration < 300*time.Millisecond {
		t.Errorf("Timeout occurred too early: %v", duration)
	}
}

// TestWaitForManifest_ReusedSession verifies that WaitForManifest handles
// reused sessions correctly (where ManifestReadyAt is already set).
func TestWaitForManifest_ReusedSession(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	session.FFmpegStartedAt = time.Now().Add(-1 * time.Second)

	// Create manifest and segment
	if err := os.WriteFile(session.ManifestPath, []byte("#EXTM3U\n"), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session.OutputDir, SegmentFilename(0)), []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to create segment: %v", err)
	}

	// First call - should set ManifestReadyAt
	if err := session.WaitForManifest(1 * time.Second); err != nil {
		t.Fatalf("First wait failed: %v", err)
	}

	firstReadyTime := session.ManifestReadyAt
	if firstReadyTime.IsZero() {
		t.Fatal("Expected ManifestReadyAt to be set after first wait")
	}

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Second call - should NOT update ManifestReadyAt (session is reused)
	if err := session.WaitForManifest(1 * time.Second); err != nil {
		t.Fatalf("Second wait failed: %v", err)
	}

	if !session.ManifestReadyAt.Equal(firstReadyTime) {
		t.Errorf("ManifestReadyAt should not change on reused session. First: %v, Second: %v",
			firstReadyTime, session.ManifestReadyAt)
	}
}

// TestWaitForInitSegment_SegmentExists verifies WaitForInitSegment returns
// immediately when the init segment already exists.
func TestWaitForInitSegment_SegmentExists(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create init segment
	initPath := filepath.Join(session.OutputDir, InitSegmentFilename)
	if err := os.WriteFile(initPath, []byte("fake init segment"), 0644); err != nil {
		t.Fatalf("Failed to create init segment: %v", err)
	}

	// Should return immediately
	start := time.Now()
	path, err := session.WaitForInitSegment(1 * time.Second)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if path != initPath {
		t.Errorf("Expected path %s, got %s", initPath, path)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Expected fast return, took %v", duration)
	}
}

// TestWaitForInitSegment_Timeout verifies WaitForInitSegment times out
// when the init segment never appears.
func TestWaitForInitSegment_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Create output directory but no init segment
	if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	start := time.Now()
	_, err := session.WaitForInitSegment(200 * time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if duration < 200*time.Millisecond {
		t.Errorf("Timeout occurred too early: %v", duration)
	}
	if duration > 400*time.Millisecond {
		t.Errorf("Timeout took too long: %v", duration)
	}
}

// TestSegmentNotification_ConditionVariable verifies that the condition variable
// properly notifies waiting goroutines when segments are generated.
func TestSegmentNotification_ConditionVariable(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := createTestSession(tempDir, logger)
	defer session.Stop()

	// Track notifications received
	notificationCount := 0
	var mu sync.Mutex

	// Start multiple waiters that will increment counter when notified
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		segNum := 30 + i
		go func(seg int) {
			defer wg.Done()
			// Use a custom wait loop to track notifications
			session.segmentMutex.Lock()
			for !session.generatedSegments[seg] {
				session.segmentCond.Wait()
				mu.Lock()
				notificationCount++
				mu.Unlock()
			}
			session.segmentMutex.Unlock()
		}(segNum)
	}

	// Wait for goroutines to start waiting
	time.Sleep(100 * time.Millisecond)

	// Mark segments and broadcast multiple times
	for i := 0; i < 5; i++ {
		segNum := 30 + i
		session.segmentMutex.Lock()
		session.generatedSegments[segNum] = true
		session.segmentCond.Broadcast()
		session.segmentMutex.Unlock()
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify we got multiple notifications (shows Broadcast is working)
		mu.Lock()
		if notificationCount < 5 {
			t.Errorf("Expected at least 5 notifications, got %d", notificationCount)
		}
		mu.Unlock()
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for condition variable notifications")
	}
}

// TestParseSegmentNumber verifies the segment number parsing logic
// used by the file system watcher.
func TestParseSegmentNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"seg_000000.ts", 0},
		{"seg_000001.ts", 1},
		{"seg_000123.ts", 123},
		{"seg_999999.ts", 999999},
		{"invalid.ts", -1},
		{"seg_abc.ts", -1},
		{"seg_123.mp4", -1},
		{"other_000123.ts", -1},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := ParseSegmentNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("ParseSegmentNumber(%q) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestSegmentFilename verifies the segment filename generation
// used by WaitForSegment.
func TestSegmentFilename(t *testing.T) {
	tests := []struct {
		segmentNum int
		expected   string
	}{
		{0, "seg_000000.ts"},
		{1, "seg_000001.ts"},
		{123, "seg_000123.ts"},
		{999999, "seg_999999.ts"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := SegmentFilename(tt.segmentNum)
			if result != tt.expected {
				t.Errorf("SegmentFilename(%d) = %q, want %q", tt.segmentNum, result, tt.expected)
			}
		})
	}
}

// Helper function to create a test session with minimal setup
func createTestSession(tempDir string, logger *slog.Logger) *TranscodeSession {
	ctx, cancel := context.WithCancel(context.Background())

	outputDir := filepath.Join(tempDir, "output")
	manifestPath := filepath.Join(outputDir, "manifest.m3u8")

	session := &TranscodeSession{
		ID:                 "test-session",
		MediaID:            123,
		Quality:            "720p",
		StartPosition:      0,
		OutputDir:          outputDir,
		ManifestPath:       manifestPath,
		CreatedAt:          time.Now(),
		LastAccessed:       time.Now(),
		SegmentDurationSec: SegmentDuration,
		ctx:                ctx,
		cancel:             cancel,
		logger:             logger,
		generatedSegments:  make(map[int]bool),
	}

	// Initialize condition variable
	session.segmentCond = sync.NewCond(&session.segmentMutex)

	return session
}
