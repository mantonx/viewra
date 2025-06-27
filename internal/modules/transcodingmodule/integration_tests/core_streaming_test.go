// Package transcodingmodule provides core streaming pipeline tests without external dependencies.
package integration_tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/events"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/pipeline"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
)

// TestCoreStreamingPipeline tests the core streaming functionality without external dependencies
func TestCoreStreamingPipeline(t *testing.T) {
	// Skip if no test video files available
	testVideoPath := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		t.Skip("No test video file available at", testVideoPath)
	}

	// Create temporary output directory
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "core_streaming_test")

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "core-streaming-test",
		Level: hclog.Debug,
	})

	t.Run("SegmentBasedEncoding", func(t *testing.T) {
		testSegmentBasedEncoding(t, testVideoPath, outputDir, logger)
	})

	t.Run("EventDrivenWorkflow", func(t *testing.T) {
		testEventDrivenWorkflow(t, tempDir, logger)
	})

	t.Run("ContentStoreIntegration", func(t *testing.T) {
		testContentStoreIntegration(t, tempDir, logger)
	})
}

func testSegmentBasedEncoding(t *testing.T, inputPath, outputDir string, logger hclog.Logger) {
	encoder := pipeline.NewStreamEncoder(outputDir, 4)

	// Create event bus for real-time notifications
	eventBus := events.NewEventBus(logger)
	encoder.SetEventBus(eventBus, "test-session", "test-content-hash")

	// Track segments and events
	segmentsProduced := 0
	eventsReceived := 0

	// Subscribe to segment events
	eventBus.Subscribe(events.SegmentReady, func(event events.SegmentEvent) error {
		eventsReceived++
		t.Logf("Event received: %s, segment index: %v", event.Type, event.Data["segment_index"])
		return nil
	})

	// Set legacy callbacks
	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			segmentsProduced++
			t.Logf("Segment %d ready: %s", segmentIndex, segmentPath)

			// Verify segment file exists and has content
			if info, err := os.Stat(segmentPath); err != nil {
				t.Errorf("Segment file missing: %s", segmentPath)
			} else if info.Size() == 0 {
				t.Errorf("Segment file is empty: %s", segmentPath)
			} else {
				t.Logf("Segment %d size: %d bytes", segmentIndex, info.Size())
			}
		},
		func(err error) {
			t.Errorf("Encoder error: %v", err)
		},
	)

	// Create streaming profiles
	profiles := []pipeline.EncodingProfile{
		{
			Name:         "720p",
			Width:        1280,
			Height:       720,
			VideoBitrate: 2500,
			Quality:      23,
		},
		{
			Name:         "480p",
			Width:        854,
			Height:       480,
			VideoBitrate: 1200,
			Quality:      25,
		},
	}

	// Start encoding with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Logf("Starting segment-based encoding of %s", inputPath)
	err := encoder.StartEncoding(ctx, inputPath, profiles)
	if err != nil {
		t.Fatalf("Failed to start encoding: %v", err)
	}

	// Wait for segments to be produced
	timeout := time.After(25 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	minSegments := 2 // Expect at least 2 segments for a 10-second video with 4-second segments

	for {
		select {
		case <-timeout:
			encoder.StopEncoding()
			if segmentsProduced < minSegments {
				t.Errorf("Insufficient segments produced: got %d, expected at least %d", segmentsProduced, minSegments)
			} else {
				t.Logf("✅ Successfully produced %d segments", segmentsProduced)
			}

			if eventsReceived < minSegments {
				t.Errorf("Insufficient events received: got %d, expected at least %d", eventsReceived, minSegments)
			} else {
				t.Logf("✅ Successfully received %d segment events", eventsReceived)
			}

			// Verify output directory structure
			verifyOutputStructure(t, outputDir)
			return

		case <-ticker.C:
			if segmentsProduced >= minSegments && eventsReceived >= minSegments {
				encoder.StopEncoding()
				t.Logf("✅ Successfully produced %d segments and received %d events", segmentsProduced, eventsReceived)
				verifyOutputStructure(t, outputDir)
				return
			}
			t.Logf("Progress: %d segments, %d events", segmentsProduced, eventsReceived)
		}
	}
}

func verifyOutputStructure(t *testing.T, outputDir string) {
	// Check for segment files
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Errorf("Failed to read output directory: %v", err)
		return
	}

	segmentCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if filepath.Ext(name) == ".mp4" {
				segmentCount++
				t.Logf("Found segment: %s", name)
			}
		}
	}

	if segmentCount == 0 {
		t.Error("No segment files found in output directory")
	} else {
		t.Logf("✅ Found %d segment files in output directory", segmentCount)
	}
}

func testEventDrivenWorkflow(t *testing.T, tempDir string, logger hclog.Logger) {
	// Create event bus
	eventBus := events.NewEventBus(logger)

	// Create content store
	contentStore, err := storage.NewContentStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create content store: %v", err)
	}

	// Create streaming event manager
	eventManager := events.NewStreamingEventManager(eventBus, contentStore, logger)
	eventManager.StartEventProcessing()

	// Create test content
	testHash := "event-test-hash-456"
	metadata := storage.ContentMetadata{
		MediaID:         "event-test-media",
		Format:          "dash",
		SegmentDuration: 4,
		StreamingStatus: "active",
	}

	// Create test source directory with segments
	sourceDir := filepath.Join(tempDir, "event-test-source")
	os.MkdirAll(sourceDir, 0755)

	testSegmentPath := filepath.Join(sourceDir, "segment_001.m4s")
	err = os.WriteFile(testSegmentPath, []byte("test segment content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test segment: %v", err)
	}

	// Store initial content
	err = contentStore.Store(testHash, sourceDir, metadata)
	if err != nil {
		t.Fatalf("Failed to store content: %v", err)
	}

	// Simulate segment ready event
	newSegmentPath := filepath.Join(tempDir, "new_segment.m4s")
	err = os.WriteFile(newSegmentPath, []byte("new segment content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new segment: %v", err)
	}

	// Publish segment ready event
	eventBus.PublishSegmentReady("event-test-session", testHash, 2, newSegmentPath)

	// Wait for event processing
	time.Sleep(500 * time.Millisecond)

	// Verify segment was added to content store
	segments, err := contentStore.GetSegments(testHash)
	if err != nil {
		t.Errorf("Failed to get segments: %v", err)
	} else if len(segments) == 0 {
		t.Error("No segments found after event processing")
	} else {
		t.Logf("✅ Found %d segments after event processing", len(segments))
	}

	// Publish manifest updated event
	eventBus.PublishManifestUpdated("event-test-session", testHash, "/test/manifest.mpd")

	// Publish stream completed event
	eventBus.PublishStreamCompleted("event-test-session", testHash, 3, 12*time.Second)

	// Stop event processing
	eventManager.StopEventProcessing()

	t.Log("✅ Event-driven workflow test completed successfully")
}

func testContentStoreIntegration(t *testing.T, tempDir string, logger hclog.Logger) {
	contentStore, err := storage.NewContentStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create content store: %v", err)
	}

	testHash := "integration-test-hash-789"

	// Create source content with streaming structure
	sourceDir := filepath.Join(tempDir, "integration-source")
	createStreamingContent(t, sourceDir)

	metadata := storage.ContentMetadata{
		MediaID:         "integration-test-media",
		Format:          "dash",
		SegmentDuration: 4,
		StreamingStatus: "active",
		QualityLevels:   []string{"720p", "480p"},
	}

	// Store content
	err = contentStore.Store(testHash, sourceDir, metadata)
	if err != nil {
		t.Fatalf("Failed to store content: %v", err)
	}

	// Verify content organization
	_, contentPath, err := contentStore.GetMetadata(testHash)
	if err != nil {
		t.Fatalf("Failed to retrieve content: %v", err)
	}

	// Check organized structure
	expectedDirs := []string{"segments", "manifests", "init", "video", "audio"}
	for _, dir := range expectedDirs {
		dirPath := filepath.Join(contentPath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Expected directory missing: %s", dir)
		} else {
			t.Logf("✅ Found expected directory: %s", dir)
		}
	}

	// Add new segment
	segmentInfo := storage.SegmentInfo{
		Index:    5,
		Profile:  "720p",
		Type:     "video",
		Duration: 4,
	}

	newSegmentPath := filepath.Join(tempDir, "new_video_segment.m4s")
	err = os.WriteFile(newSegmentPath, []byte("new video segment"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new segment: %v", err)
	}

	err = contentStore.AddSegment(testHash, newSegmentPath, segmentInfo)
	if err != nil {
		t.Errorf("Failed to add segment: %v", err)
	} else {
		t.Log("✅ Successfully added new segment")
	}

	// Verify all segments
	segments, err := contentStore.GetSegments(testHash)
	if err != nil {
		t.Errorf("Failed to get segments: %v", err)
	} else {
		t.Logf("✅ Total segments found: %d", len(segments))
		for i, segment := range segments {
			t.Logf("  Segment %d: %s", i+1, filepath.Base(segment))
		}
	}

	t.Log("✅ Content store integration test completed successfully")
}

func createStreamingContent(t *testing.T, sourceDir string) {
	os.MkdirAll(sourceDir, 0755)

	// Create test files representing streaming content
	files := map[string]string{
		"manifest.mpd":   "<?xml version=\"1.0\"?><MPD>test manifest</MPD>",
		"init_video.mp4": "init video segment",
		"init_audio.mp4": "init audio segment",
		"video_001.m4s":  "video segment 1",
		"video_002.m4s":  "video segment 2",
		"audio_001.m4s":  "audio segment 1",
		"audio_002.m4s":  "audio segment 2",
		"playlist.m3u8":  "#EXTM3U\n#EXT-X-VERSION:3\n",
	}

	for filename, content := range files {
		filePath := filepath.Join(sourceDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}
}
