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
)

func TestWaitForSegment(t *testing.T) {
	tests := []struct {
		name           string
		segmentNum     int
		timeout        time.Duration
		preMarkSegment bool
		markAfterDelay time.Duration
		expectError    bool
		expectFast     bool
	}{
		{
			name:           "segment already exists",
			segmentNum:     5,
			timeout:        1 * time.Second,
			preMarkSegment: true,
			expectFast:     true,
		},
		{
			name:           "wait for segment to appear",
			segmentNum:     10,
			timeout:        2 * time.Second,
			markAfterDelay: 100 * time.Millisecond,
		},
		{
			name:        "timeout when segment never appears",
			segmentNum:  99,
			timeout:     200 * time.Millisecond,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createWatcherTestSession(t)
			defer session.Stop()

			if tt.preMarkSegment {
				session.segmentMutex.Lock()
				session.generatedSegments[tt.segmentNum] = true
				session.segmentMutex.Unlock()
			}

			resultChan := make(chan error, 1)
			start := time.Now()

			go func() {
				if tt.markAfterDelay > 0 {
					time.Sleep(tt.markAfterDelay)
					session.segmentMutex.Lock()
					session.generatedSegments[tt.segmentNum] = true
					session.segmentCond.Broadcast()
					session.segmentMutex.Unlock()
				}
			}()

			path, err := session.WaitForSegment(tt.segmentNum, tt.timeout)
			resultChan <- err
			duration := time.Since(start)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if duration < tt.timeout {
					t.Errorf("Timeout too early: %v", duration)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedPath := filepath.Join(session.OutputDir, SegmentFilename(tt.segmentNum))
			if path != expectedPath {
				t.Errorf("Expected path %s, got %s", expectedPath, path)
			}

			if tt.expectFast && duration > 100*time.Millisecond {
				t.Errorf("Expected fast return, took %v", duration)
			}
		})
	}
}

func TestWaitForSegment_MultipleWaiters(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

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

	time.Sleep(100 * time.Millisecond)

	session.segmentMutex.Lock()
	for i := 0; i < 3; i++ {
		session.generatedSegments[20+i] = true
	}
	session.segmentCond.Broadcast()
	session.segmentMutex.Unlock()

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
		t.Fatal("Timeout waiting for all goroutines")
	}
}

func TestWaitForManifest(t *testing.T) {
	tests := []struct {
		name            string
		startPosition   float64
		segmentDuration int
		expectedSegment int
		preCreate       bool
		createAfterMs   time.Duration
		staleFiles      bool
		expectError     bool
	}{
		{
			name:            "manifest and segment exist",
			expectedSegment: 0,
			preCreate:       true,
		},
		{
			name:            "wait for creation",
			expectedSegment: 0,
			createAfterMs:   100 * time.Millisecond,
		},
		{
			name:        "timeout when files never appear",
			expectError: true,
		},
		{
			name:            "start position 10s",
			startPosition:   10,
			segmentDuration: 2,
			expectedSegment: 5,
			preCreate:       true,
		},
		{
			name:            "start position 37s",
			startPosition:   37,
			segmentDuration: 2,
			expectedSegment: 18,
			preCreate:       true,
		},
		{
			name:            "stale manifest rejected",
			expectedSegment: 0,
			staleFiles:      true,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createWatcherTestSession(t)
			defer session.Stop()

			if tt.segmentDuration > 0 {
				session.SegmentDurationSec = tt.segmentDuration
			}
			session.StartPosition = tt.startPosition

			if err := os.MkdirAll(session.OutputDir, 0755); err != nil {
				t.Fatalf("Failed to create output dir: %v", err)
			}

			timeout := 200 * time.Millisecond
			if !tt.expectError {
				timeout = 2 * time.Second
			}

			createFiles := func() {
				os.WriteFile(session.ManifestPath, []byte("#EXTM3U\n"), 0644)
				segPath := filepath.Join(session.OutputDir, SegmentFilename(tt.expectedSegment))
				os.WriteFile(segPath, []byte("data"), 0644)
			}

			if tt.staleFiles {
				createFiles()
				time.Sleep(10 * time.Millisecond)
				session.FFmpegStartedAt = time.Now()
			} else if tt.preCreate {
				session.FFmpegStartedAt = time.Now().Add(-1 * time.Second)
				createFiles()
			} else if tt.createAfterMs > 0 {
				session.FFmpegStartedAt = time.Now()
				go func() {
					time.Sleep(tt.createAfterMs)
					createFiles()
				}()
			} else {
				session.FFmpegStartedAt = time.Now()
			}

			start := time.Now()
			err := session.WaitForManifest(timeout)
			duration := time.Since(start)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if duration < timeout {
					t.Errorf("Timeout too early: %v", duration)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if session.ManifestReadyAt.IsZero() {
				t.Error("ManifestReadyAt should be set")
			}
		})
	}
}

func TestWaitForManifest_ReusedSession(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	session.FFmpegStartedAt = time.Now().Add(-1 * time.Second)
	os.WriteFile(session.ManifestPath, []byte("#EXTM3U\n"), 0644)
	os.WriteFile(filepath.Join(session.OutputDir, SegmentFilename(0)), []byte("data"), 0644)

	if err := session.WaitForManifest(1 * time.Second); err != nil {
		t.Fatalf("First wait failed: %v", err)
	}

	firstReadyTime := session.ManifestReadyAt
	time.Sleep(50 * time.Millisecond)

	if err := session.WaitForManifest(1 * time.Second); err != nil {
		t.Fatalf("Second wait failed: %v", err)
	}

	if !session.ManifestReadyAt.Equal(firstReadyTime) {
		t.Errorf("ManifestReadyAt changed on reused session")
	}
}

func TestWaitForInitSegment(t *testing.T) {
	tests := []struct {
		name        string
		preCreate   bool
		expectError bool
	}{
		{name: "init segment exists", preCreate: true},
		{name: "timeout when not found", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createWatcherTestSession(t)
			defer session.Stop()

			os.MkdirAll(session.OutputDir, 0755)

			if tt.preCreate {
				initPath := filepath.Join(session.OutputDir, InitSegmentFilename)
				os.WriteFile(initPath, []byte("init"), 0644)
			}

			timeout := 200 * time.Millisecond
			path, err := session.WaitForInitSegment(timeout)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedPath := filepath.Join(session.OutputDir, InitSegmentFilename)
			if path != expectedPath {
				t.Errorf("Expected path %s, got %s", expectedPath, path)
			}
		})
	}
}

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
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := ParseSegmentNumber(tt.filename); got != tt.expected {
				t.Errorf("ParseSegmentNumber(%q) = %d, want %d", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestSegmentFilename(t *testing.T) {
	tests := []struct {
		num      int
		expected string
	}{
		{0, "seg_000000.ts"},
		{1, "seg_000001.ts"},
		{123, "seg_000123.ts"},
		{999999, "seg_999999.ts"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := SegmentFilename(tt.num); got != tt.expected {
				t.Errorf("SegmentFilename(%d) = %q, want %q", tt.num, got, tt.expected)
			}
		})
	}
}

func TestWatchSegments_DetectsNewSegments(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	go session.watchSegments()
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		segPath := filepath.Join(session.OutputDir, SegmentFilename(i))
		os.WriteFile(segPath, []byte("data"), 0644)

		detected := false
		for start := time.Now(); time.Since(start) < 1*time.Second; {
			session.segmentMutex.RLock()
			detected = session.generatedSegments[i]
			session.segmentMutex.RUnlock()
			if detected {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if !detected {
			t.Errorf("Segment %d not detected", i)
		}
	}
}

func TestWatchSegments_IgnoresNonSegmentFiles(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	go session.watchSegments()
	time.Sleep(100 * time.Millisecond)

	nonSegmentFiles := []string{"playlist.m3u8", "other.ts", "seg_5.mp4", "readme.txt"}
	for _, file := range nonSegmentFiles {
		os.WriteFile(filepath.Join(session.OutputDir, file), []byte("data"), 0644)
	}

	time.Sleep(200 * time.Millisecond)

	session.segmentMutex.RLock()
	count := len(session.generatedSegments)
	session.segmentMutex.RUnlock()

	if count > 0 {
		t.Errorf("Expected no segments detected, got %d", count)
	}
}

func TestWatchSegments_StopsOnContextCancel(t *testing.T) {
	session := createWatcherTestSession(t)
	os.MkdirAll(session.OutputDir, 0755)

	done := make(chan struct{})
	go func() {
		session.watchSegments()
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	session.cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("watchSegments did not exit after context cancel")
	}
}

func TestWatchSegments_BroadcastsOnNewSegment(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	go session.watchSegments()
	time.Sleep(100 * time.Millisecond)

	waitDone := make(chan error, 1)
	go func() {
		_, err := session.WaitForSegment(5, 2*time.Second)
		waitDone <- err
	}()

	time.Sleep(100 * time.Millisecond)
	segPath := filepath.Join(session.OutputDir, SegmentFilename(5))
	os.WriteFile(segPath, []byte("data"), 0644)

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("WaitForSegment failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForSegment did not complete after segment creation")
	}
}

func TestWatchSegments_DuplicateSegmentIgnored(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	go session.watchSegments()
	time.Sleep(100 * time.Millisecond)

	segPath := filepath.Join(session.OutputDir, SegmentFilename(10))
	os.WriteFile(segPath, []byte("initial"), 0644)
	time.Sleep(200 * time.Millisecond)

	session.segmentMutex.RLock()
	firstMark := session.generatedSegments[10]
	session.segmentMutex.RUnlock()

	if !firstMark {
		t.Fatal("Segment not initially detected")
	}

	for i := 0; i < 3; i++ {
		os.WriteFile(segPath, []byte(fmt.Sprintf("modified %d", i)), 0644)
		time.Sleep(50 * time.Millisecond)
	}

	session.segmentMutex.RLock()
	stillMarked := session.generatedSegments[10]
	session.segmentMutex.RUnlock()

	if !stillMarked {
		t.Error("Segment should remain marked after modifications")
	}
}

func TestWatchSegmentsPolling_DetectsSegments(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)
	go session.watchSegmentsPolling()
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		segPath := filepath.Join(session.OutputDir, SegmentFilename(i))
		os.WriteFile(segPath, []byte("data"), 0644)
	}

	time.Sleep(300 * time.Millisecond)

	session.segmentMutex.RLock()
	for i := 0; i < 3; i++ {
		if !session.generatedSegments[i] {
			t.Errorf("Segment %d not detected", i)
		}
	}
	session.segmentMutex.RUnlock()
}

func TestWatchSegmentsPolling_DetectsPreexistingSegments(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)

	for i := 0; i < 5; i++ {
		segPath := filepath.Join(session.OutputDir, SegmentFilename(i))
		os.WriteFile(segPath, []byte("preexisting"), 0644)
	}

	go session.watchSegmentsPolling()
	time.Sleep(200 * time.Millisecond)

	session.segmentMutex.RLock()
	for i := 0; i < 5; i++ {
		if !session.generatedSegments[i] {
			t.Errorf("Preexisting segment %d not detected", i)
		}
	}
	session.segmentMutex.RUnlock()
}

func TestWatchSegmentsPolling_StopsOnContextCancel(t *testing.T) {
	session := createWatcherTestSession(t)
	os.MkdirAll(session.OutputDir, 0755)

	done := make(chan struct{})
	go func() {
		session.watchSegmentsPolling()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	session.cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watchSegmentsPolling did not exit after context cancel")
	}
}

func TestWatchSegmentsPolling_IgnoresNonSegmentFiles(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	os.MkdirAll(session.OutputDir, 0755)

	nonSegmentFiles := []string{"playlist.m3u8", "other.ts", "seg_5.mp4"}
	for _, file := range nonSegmentFiles {
		os.WriteFile(filepath.Join(session.OutputDir, file), []byte("data"), 0644)
	}

	go session.watchSegmentsPolling()
	time.Sleep(400 * time.Millisecond)

	session.segmentMutex.RLock()
	count := len(session.generatedSegments)
	session.segmentMutex.RUnlock()

	if count > 0 {
		t.Errorf("Expected no segments, got %d", count)
	}
}

func TestWatchSegments_FallbackToPolling(t *testing.T) {
	session := createWatcherTestSession(t)
	defer session.Stop()

	session.OutputDir = filepath.Join(session.OutputDir, "nonexistent")
	go session.watchSegments()
	time.Sleep(100 * time.Millisecond)

	os.MkdirAll(session.OutputDir, 0755)
	segPath := filepath.Join(session.OutputDir, SegmentFilename(0))
	os.WriteFile(segPath, []byte("data"), 0644)

	time.Sleep(300 * time.Millisecond)

	session.segmentMutex.RLock()
	detected := session.generatedSegments[0]
	session.segmentMutex.RUnlock()

	if !detected {
		t.Error("Segment not detected by polling fallback")
	}
}

func createWatcherTestSession(t *testing.T) *TranscodeSession {
	t.Helper()
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
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

	session.segmentCond = sync.NewCond(&session.segmentMutex)
	return session
}
