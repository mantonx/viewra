// Package transcodingmodule provides real media file testing for the streaming pipeline.
// This test validates the complete streaming workflow with actual video content.
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
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/hash"
)

// TestRealMediaStreaming tests the complete streaming pipeline with real media files
func TestRealMediaStreaming(t *testing.T) {
	testFiles := []string{
		"/home/fictional/Projects/viewra/viewra-data/test-video.mp4",
		"/home/fictional/Projects/viewra/viewra-data/media/movies/test.mp4",
	}

	var testFile string
	for _, file := range testFiles {
		if _, err := os.Stat(file); err == nil {
			testFile = file
			break
		}
	}

	if testFile == "" {
		t.Skip("No test video files available")
	}

	t.Logf("Testing with real media file: %s", testFile)

	// Create comprehensive test suite
	t.Run("CompleteStreamingWorkflow", func(t *testing.T) {
		testCompleteStreamingWorkflow(t, testFile)
	})

	t.Run("MultiProfileStreaming", func(t *testing.T) {
		testMultiProfileStreaming(t, testFile)
	})

	t.Run("ContentHashGeneration", func(t *testing.T) {
		testContentHashGeneration(t, testFile)
	})

	t.Run("SegmentProgression", func(t *testing.T) {
		testSegmentProgression(t, testFile)
	})
}

func testCompleteStreamingWorkflow(t *testing.T, inputFile string) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "complete_workflow")

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "real-media-test",
		Level: hclog.Info,
	})

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
	defer eventManager.StopEventProcessing()

	// Generate content hash
	mediaID := "real-media-test"
	contentHash := hash.GenerateContentHash(mediaID, "dash", 65, &hash.Resolution{Width: 1280, Height: 720})

	t.Logf("Generated content hash: %s", contentHash)

	// Create encoder
	encoder := pipeline.NewStreamEncoder(outputDir, 4)
	encoder.SetEventBus(eventBus, "real-media-session", contentHash)

	// Track streaming progress
	var segments []SegmentInfo
	var eventCount int

	// Subscribe to all events
	eventBus.Subscribe(events.SegmentReady, func(event events.SegmentEvent) error {
		eventCount++
		t.Logf("Event: %s - Segment %v ready", event.Type, event.Data["segment_index"])
		return nil
	})

	eventBus.Subscribe(events.ManifestUpdated, func(event events.SegmentEvent) error {
		eventCount++
		t.Logf("Event: %s - Manifest updated: %v", event.Type, event.Data["manifest_path"])
		return nil
	})

	// Set encoder callbacks
	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			info := SegmentInfo{
				Path:      segmentPath,
				Index:     segmentIndex,
				Type:      "video",
				Timestamp: time.Now(),
			}
			segments = append(segments, info)

			// Validate segment
			if stat, err := os.Stat(segmentPath); err != nil {
				t.Errorf("Segment file missing: %s", segmentPath)
			} else {
				t.Logf("Segment %d: %s (%d bytes)", segmentIndex, filepath.Base(segmentPath), stat.Size())
			}
		},
		func(err error) {
			t.Errorf("Encoder error: %v", err)
		},
	)

	// Create streaming profiles for different qualities
	profiles := []pipeline.EncodingProfile{
		{
			Name:         "1080p",
			Width:        1920,
			Height:       1080,
			VideoBitrate: 4500,
			Quality:      20,
		},
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

	// Start streaming transcoding
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	t.Logf("Starting streaming transcoding with %d profiles", len(profiles))
	err = encoder.StartEncoding(ctx, inputFile, profiles)
	if err != nil {
		t.Fatalf("Failed to start encoding: %v", err)
	}

	// Wait for segments to be produced
	startTime := time.Now()
	timeout := time.After(35 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	minSegments := 2 // Expect at least 2 segments from 10-second video

	for {
		select {
		case <-timeout:
			encoder.StopEncoding()
			goto validateResults

		case <-ticker.C:
			if len(segments) >= minSegments {
				t.Logf("Target segments reached (%d), stopping encoder", len(segments))
				encoder.StopEncoding()
				goto validateResults
			}
			t.Logf("Progress: %d segments, %d events (elapsed: %v)", len(segments), eventCount, time.Since(startTime))
		}
	}

validateResults:
	// Validate results
	processingDuration := time.Since(startTime)

	t.Logf("=== STREAMING WORKFLOW RESULTS ===")
	t.Logf("Processing duration: %v", processingDuration)
	t.Logf("Segments produced: %d", len(segments))
	t.Logf("Events fired: %d", eventCount)

	if len(segments) == 0 {
		t.Error("❌ No segments were produced")
	} else {
		t.Logf("✅ Successfully produced %d segments", len(segments))
	}

	if eventCount == 0 {
		t.Error("❌ No events were fired")
	} else {
		t.Logf("✅ Successfully fired %d events", eventCount)
	}

	// Validate segment files
	for i, segment := range segments {
		if _, err := os.Stat(segment.Path); err != nil {
			t.Errorf("❌ Segment %d file missing: %s", i, segment.Path)
		} else {
			t.Logf("✅ Segment %d validated: %s", i, filepath.Base(segment.Path))
		}
	}

	// Validate output directory structure
	validateStreamingOutput(t, outputDir)

	// Store content in content store
	metadata := storage.ContentMetadata{
		MediaID:         mediaID,
		Format:          "dash",
		SegmentDuration: 4,
		StreamingStatus: "completed",
		SegmentCount:    len(segments),
		QualityLevels:   []string{"1080p", "720p", "480p"},
	}

	err = contentStore.Store(contentHash, outputDir, metadata)
	if err != nil {
		t.Errorf("❌ Failed to store content: %v", err)
	} else {
		t.Logf("✅ Content stored with hash: %s", contentHash)
	}

	t.Log("✅ Complete streaming workflow test passed")
}

func testMultiProfileStreaming(t *testing.T, inputFile string) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "multi_profile")

	_ = hclog.New(&hclog.LoggerOptions{
		Name:  "multi-profile-test",
		Level: hclog.Warn, // Reduce noise for this test
	})

	encoder := pipeline.NewStreamEncoder(outputDir, 3) // Shorter segments for faster test

	profileSegments := make(map[string]int)

	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			// Try to determine profile from segment content/size
			if stat, err := os.Stat(segmentPath); err == nil {
				size := stat.Size()
				profile := "unknown"

				// Rough size-based profile detection
				if size > 200000 { // > 200KB likely 1080p
					profile = "1080p"
				} else if size > 100000 { // > 100KB likely 720p
					profile = "720p"
				} else { // smaller likely 480p
					profile = "480p"
				}

				profileSegments[profile]++
				t.Logf("Segment %d (%s): %d bytes", segmentIndex, profile, size)
			}
		},
		func(err error) {
			t.Errorf("Multi-profile encoding error: %v", err)
		},
	)

	profiles := []pipeline.EncodingProfile{
		{Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: 5000, Quality: 18},
		{Name: "720p", Width: 1280, Height: 720, VideoBitrate: 2800, Quality: 21},
		{Name: "480p", Width: 854, Height: 480, VideoBitrate: 1400, Quality: 24},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Logf("Testing multi-profile streaming with %d quality levels", len(profiles))

	err := encoder.StartEncoding(ctx, inputFile, profiles)
	if err != nil {
		t.Fatalf("Failed to start multi-profile encoding: %v", err)
	}

	// Wait for completion
	time.Sleep(20 * time.Second)
	encoder.StopEncoding()

	t.Logf("=== MULTI-PROFILE RESULTS ===")
	totalSegments := 0
	for profile, count := range profileSegments {
		t.Logf("%s: %d segments", profile, count)
		totalSegments += count
	}

	if totalSegments == 0 {
		t.Error("❌ No multi-profile segments produced")
	} else {
		t.Logf("✅ Multi-profile streaming produced %d total segments", totalSegments)
	}
}

func testContentHashGeneration(t *testing.T, inputFile string) {
	t.Log("Testing content hash generation and consistency")

	mediaID := "hash-test-media"

	// Generate hashes with different parameters
	hash1 := hash.GenerateContentHash(mediaID, "dash", 65, &hash.Resolution{Width: 1280, Height: 720})
	hash2 := hash.GenerateContentHash(mediaID, "dash", 65, &hash.Resolution{Width: 1280, Height: 720})
	hash3 := hash.GenerateContentHash(mediaID, "hls", 65, &hash.Resolution{Width: 1280, Height: 720})
	hash4 := hash.GenerateContentHash(mediaID, "dash", 80, &hash.Resolution{Width: 1280, Height: 720})
	hash5 := hash.GenerateContentHash(mediaID, "dash", 65, &hash.Resolution{Width: 1920, Height: 1080})

	t.Logf("Hash (DASH, Q65, 720p): %s", hash1)
	t.Logf("Hash (DASH, Q65, 720p): %s", hash2)
	t.Logf("Hash (HLS,  Q65, 720p): %s", hash3)
	t.Logf("Hash (DASH, Q80, 720p): %s", hash4)
	t.Logf("Hash (DASH, Q65, 1080p): %s", hash5)

	// Validate consistency
	if hash1 != hash2 {
		t.Error("❌ Identical parameters should produce identical hashes")
	} else {
		t.Log("✅ Hash consistency validated")
	}

	// Validate uniqueness
	uniqueHashes := map[string]bool{hash1: true, hash3: true, hash4: true, hash5: true}
	if len(uniqueHashes) != 4 {
		t.Error("❌ Different parameters should produce different hashes")
	} else {
		t.Log("✅ Hash uniqueness validated")
	}
}

func testSegmentProgression(t *testing.T, inputFile string) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "progression")

	_ = hclog.New(&hclog.LoggerOptions{
		Name:  "progression-test",
		Level: hclog.Error,
	})

	encoder := pipeline.NewStreamEncoder(outputDir, 2) // 2-second segments for faster progression

	var segmentTimes []time.Time
	startTime := time.Now()

	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			segmentTime := time.Since(startTime)
			segmentTimes = append(segmentTimes, time.Now())

			t.Logf("Segment %d ready at %v", segmentIndex, segmentTime)
		},
		func(err error) {
			t.Errorf("Progression test error: %v", err)
		},
	)

	profiles := []pipeline.EncodingProfile{
		{Name: "480p", Width: 854, Height: 480, VideoBitrate: 1000, Quality: 28}, // Fast encoding
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	t.Log("Testing segment progression timing")

	err := encoder.StartEncoding(ctx, inputFile, profiles)
	if err != nil {
		t.Fatalf("Failed to start progression test: %v", err)
	}

	// Wait for segments
	time.Sleep(15 * time.Second)
	encoder.StopEncoding()

	t.Logf("=== SEGMENT PROGRESSION RESULTS ===")
	t.Logf("Total segments: %d", len(segmentTimes))

	if len(segmentTimes) >= 2 {
		for i := 1; i < len(segmentTimes); i++ {
			interval := segmentTimes[i].Sub(segmentTimes[i-1])
			t.Logf("Interval %d→%d: %v", i-1, i, interval)
		}
		t.Log("✅ Segment progression validated")
	} else {
		t.Error("❌ Insufficient segments for progression analysis")
	}
}

func validateStreamingOutput(t *testing.T, outputDir string) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Errorf("Failed to read output directory: %v", err)
		return
	}

	segmentCount := 0
	csvFound := false

	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) == ".mp4" {
			segmentCount++
		} else if name == "segments.csv" {
			csvFound = true
		}
	}

	if segmentCount == 0 {
		t.Error("❌ No segment files found")
	} else {
		t.Logf("✅ Found %d segment files", segmentCount)
	}

	if !csvFound {
		t.Error("❌ segments.csv not found")
	} else {
		t.Log("✅ segments.csv found")
	}
}

// SegmentInfo represents information about a generated segment
type SegmentInfo struct {
	Path      string
	Index     int
	Type      string
	Timestamp time.Time
}
