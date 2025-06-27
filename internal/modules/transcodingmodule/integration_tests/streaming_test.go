// Package integration_tests provides streaming pipeline testing utilities.
// This file contains integration tests for the streaming-first transcoding pipeline.
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

// TestStreamingPipeline tests the end-to-end streaming pipeline
func TestStreamingPipeline(t *testing.T) {
	// Skip if no test video files available
	testVideoPath := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		t.Skip("No test video file available at", testVideoPath)
	}

	// Create temporary output directory
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "streaming_test")

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "streaming-test",
		Level: hclog.Debug,
	})

	// Test streaming pipeline components
	t.Run("StreamEncoder", func(t *testing.T) {
		testStreamEncoder(t, testVideoPath, outputDir, logger)
	})

	t.Run("StreamPackager", func(t *testing.T) {
		testStreamPackager(t, outputDir, logger)
	})

	t.Run("EventSystem", func(t *testing.T) {
		testEventSystem(t, logger)
	})

	t.Run("ContentStore", func(t *testing.T) {
		testContentStore(t, tempDir, logger)
	})
}

func testStreamEncoder(t *testing.T, inputPath, outputDir string, logger hclog.Logger) {
	encoder := pipeline.NewStreamEncoder(outputDir, 4) // 4-second segments

	// Set up event tracking
	segmentsProduced := 0
	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			segmentsProduced++
			t.Logf("Segment %d ready: %s", segmentIndex, segmentPath)
		},
		func(err error) {
			t.Errorf("Encoder error: %v", err)
		},
	)

	// Create encoding profiles
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

	// Start encoding
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := encoder.StartEncoding(ctx, inputPath, profiles)
	if err != nil {
		t.Fatalf("Failed to start encoding: %v", err)
	}

	// Wait for some segments to be produced
	timeout := time.After(20 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			encoder.StopEncoding()
			if segmentsProduced == 0 {
				t.Error("No segments were produced")
			} else {
				t.Logf("Successfully produced %d segments", segmentsProduced)
			}
			return
		case <-ticker.C:
			if segmentsProduced >= 3 {
				encoder.StopEncoding()
				t.Logf("Successfully produced %d segments", segmentsProduced)
				return
			}
		}
	}
}

func testStreamPackager(t *testing.T, outputDir string, logger hclog.Logger) {
	packager := pipeline.NewStreamPackager(outputDir, "http://localhost:8080/test/")

	manifestUpdates := 0
	packager.SetCallbacks(
		func(manifestPath string) {
			manifestUpdates++
			t.Logf("Manifest updated: %s", manifestPath)
		},
		func(segmentPath string) {
			t.Logf("Segment packaged: %s", segmentPath)
		},
		func(err error) {
			t.Errorf("Packager error: %v", err)
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := packager.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start packager: %v", err)
		return
	}

	// Simulate segment arrival
	segmentInfo := pipeline.SegmentInfo{
		Path:      filepath.Join(outputDir, "test_segment.mp4"),
		Index:     1,
		Profile:   "720p",
		Type:      "video",
		Timestamp: time.Now(),
	}

	// Create a dummy segment file
	if err := os.WriteFile(segmentInfo.Path, []byte("dummy segment data"), 0644); err != nil {
		t.Errorf("Failed to create test segment: %v", err)
		return
	}

	err = packager.QueueSegment(segmentInfo)
	if err != nil {
		t.Errorf("Failed to queue segment: %v", err)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	packager.Stop()

	if manifestUpdates == 0 {
		t.Error("No manifest updates were triggered")
	}
}

func testEventSystem(t *testing.T, logger hclog.Logger) {
	eventBus := events.NewEventBus(logger)

	eventsReceived := 0
	eventBus.Subscribe(events.SegmentReady, func(event events.SegmentEvent) error {
		eventsReceived++
		t.Logf("Received event: %s for session %s", event.Type, event.SessionID)
		return nil
	})

	// Publish test events
	eventBus.PublishSegmentReady("test-session", "test-hash", 1, "/test/segment.mp4")
	eventBus.PublishManifestUpdated("test-session", "test-hash", "/test/manifest.mpd")

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	eventBus.Stop()

	if eventsReceived == 0 {
		t.Error("No events were received")
	} else {
		t.Logf("Successfully received %d events", eventsReceived)
	}
}

func testContentStore(t *testing.T, tempDir string, logger hclog.Logger) {
	contentStore, err := storage.NewContentStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create content store: %v", err)
	}

	// Test content storage
	testHash := "test-content-hash-123"
	metadata := storage.ContentMetadata{
		MediaID:         "test-media",
		Format:          "dash",
		SegmentDuration: 4,
		StreamingStatus: "active",
	}

	// Create test content directory
	sourceDir := filepath.Join(tempDir, "test-source")
	os.MkdirAll(sourceDir, 0755)

	// Create test files
	testFiles := []string{
		"manifest.mpd",
		"init.mp4",
		"segment_001.m4s",
		"segment_002.m4s",
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(sourceDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Errorf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Store content
	err = contentStore.Store(testHash, sourceDir, metadata)
	if err != nil {
		t.Errorf("Failed to store content: %v", err)
		return
	}

	// Retrieve content
	retrievedMeta, _, err := contentStore.Get(testHash)
	if err != nil {
		t.Errorf("Failed to retrieve content: %v", err)
		return
	}

	if retrievedMeta == nil {
		t.Error("Retrieved metadata is nil")
		return
	}

	// Check if content exists
	if !contentStore.Exists(testHash) {
		t.Error("Content should exist after storage")
	}

	// Test segment addition
	segmentInfo := storage.SegmentInfo{
		Index:    3,
		Profile:  "720p",
		Type:     "video",
		Duration: 4,
	}

	newSegmentPath := filepath.Join(tempDir, "new_segment.m4s")
	err = os.WriteFile(newSegmentPath, []byte("new segment"), 0644)
	if err != nil {
		t.Errorf("Failed to create new segment: %v", err)
		return
	}

	err = contentStore.AddSegment(testHash, newSegmentPath, segmentInfo)
	if err != nil {
		t.Errorf("Failed to add segment: %v", err)
		return
	}

	// Get segments
	segments, err := contentStore.GetSegments(testHash)
	if err != nil {
		t.Errorf("Failed to get segments: %v", err)
		return
	}

	if len(segments) == 0 {
		t.Error("No segments found")
	} else {
		t.Logf("Found %d segments", len(segments))
	}

	t.Logf("Content store test completed successfully")
}

// BenchmarkStreamingPipeline benchmarks the streaming pipeline performance
func BenchmarkStreamingPipeline(b *testing.B) {
	testVideoPath := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		b.Skip("No test video file available")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tempDir := b.TempDir()
		outputDir := filepath.Join(tempDir, "benchmark")

		encoder := pipeline.NewStreamEncoder(outputDir, 2) // Shorter segments for faster benchmark

		profiles := []pipeline.EncodingProfile{
			{
				Name:         "480p",
				Width:        854,
				Height:       480,
				VideoBitrate: 800,
				Quality:      28, // Faster encoding
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		encoder.StartEncoding(ctx, testVideoPath, profiles)

		// Wait for a few segments
		time.Sleep(5 * time.Second)
		encoder.StopEncoding()

		cancel()
	}
}
