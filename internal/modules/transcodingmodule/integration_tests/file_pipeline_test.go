// Package integration_tests provides integration tests for file-based transcoding.
// These tests validate the complete transcoding workflow with actual media files.
package integration_tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/pipeline"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	plugins "github.com/mantonx/viewra/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFilePipelineTranscoding tests the complete file-based transcoding workflow
func TestFilePipelineTranscoding(t *testing.T) {
	// Skip if no test files available
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

	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "file-pipeline-test",
		Level: hclog.Debug,
	})

	// Create session store (mock)
	sessionStore := session.NewSessionStore(nil, logger)

	// Create content store
	contentStore, err := storage.NewContentStore(tempDir, logger)
	require.NoError(t, err)

	// Create file pipeline
	filePipeline := pipeline.NewFilePipeline(logger, sessionStore, contentStore, tempDir)

	t.Run("BasicTranscoding", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Create transcode request
		req := plugins.TranscodeRequest{
			MediaID:    "test-media-1",
			InputPath:  testFile,
			Container:  "mp4",
			VideoCodec: "libx264",
			AudioCodec: "aac",
			Resolution: &plugins.Resolution{
				Width:  1280,
				Height: 720,
			},
		}

		// Start transcoding
		handle, err := filePipeline.Transcode(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, handle)
		assert.NotEmpty(t, handle.SessionID)

		// Wait for completion or timeout
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		completed := false
		for {
			select {
			case <-ticker.C:
				progress, err := filePipeline.GetProgress(handle.SessionID)
				require.NoError(t, err)
				
				t.Logf("Progress: %.0f%%", progress.PercentComplete*100)
				
				// Check session status from handle
				session, err := sessionStore.GetSession(handle.SessionID)
				require.NoError(t, err)
				
				if session.Status == "completed" {
					completed = true
					goto done
				} else if session.Status == "failed" {
					t.Fatal("Transcoding failed")
				}
			case <-ctx.Done():
				t.Fatal("Transcoding timed out")
			}
		}
	done:
		assert.True(t, completed, "Transcoding should complete successfully")
	})

	t.Run("ContentDeduplication", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Create same transcode request
		req := plugins.TranscodeRequest{
			MediaID:    "test-media-2",
			InputPath:  testFile,
			Container:  "mp4",
			VideoCodec: "libx264",
			AudioCodec: "aac",
			Resolution: &plugins.Resolution{
				Width:  1280,
				Height: 720,
			},
		}

		// Start first transcoding
		_, err = filePipeline.Transcode(ctx, req)
		require.NoError(t, err)

		// Start second transcoding with same parameters
		req.MediaID = "test-media-3"
		handle2, err := filePipeline.Transcode(ctx, req)
		require.NoError(t, err)

		// Second should complete immediately (reusing content)
		session, err := sessionStore.GetSession(handle2.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "completed", session.Status, "Second transcode should reuse existing content")
		
		t.Logf("Content deduplication working - second request completed immediately")
	})

	t.Run("DifferentFormats", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		formats := []struct {
			name       string
			container  string
			videoCodec string
			resolution string
			width      int
			height     int
		}{
			{"480p MP4", "mp4", "libx264", "480p", 854, 480},
			{"1080p MP4", "mp4", "libx264", "1080p", 1920, 1080},
			{"MKV H265", "mkv", "libx265", "720p", 1280, 720},
		}

		for _, format := range formats {
			t.Run(format.name, func(t *testing.T) {
				req := plugins.TranscodeRequest{
					MediaID:    "test-" + format.name,
					InputPath:  testFile,
					Container:  format.container,
					VideoCodec: format.videoCodec,
					AudioCodec: "aac",
					Resolution: &plugins.Resolution{
						Width:  format.width,
						Height: format.height,
					},
				}

				handle, err := filePipeline.Transcode(ctx, req)
				require.NoError(t, err)
				assert.NotNil(t, handle)
				
				t.Logf("Started transcoding for %s - Session: %s", format.name, handle.SessionID)
			})
		}
	})
}

// TestFilePipelineErrors tests error handling in the file pipeline
func TestFilePipelineErrors(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "file-pipeline-error-test",
		Level: hclog.Debug,
	})

	sessionStore := session.NewSessionStore(nil, logger)
	contentStore, err := storage.NewContentStore(tempDir, logger)
	require.NoError(t, err)

	filePipeline := pipeline.NewFilePipeline(logger, sessionStore, contentStore, tempDir)

	t.Run("InvalidInputFile", func(t *testing.T) {
		ctx := context.Background()
		
		req := plugins.TranscodeRequest{
			MediaID:    "test-invalid",
			InputPath:  "/nonexistent/file.mp4",
			Container:  "mp4",
			VideoCodec: "libx264",
			AudioCodec: "aac",
		}

		handle, err := filePipeline.Transcode(ctx, req)
		require.NoError(t, err) // Pipeline starts regardless
		assert.NotNil(t, handle)

		// Wait a bit for FFmpeg to fail
		time.Sleep(3 * time.Second)

		// Check session status - should be failed
		session, err := sessionStore.GetSession(handle.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "failed", session.Status)
	})

	t.Run("StopTranscoding", func(t *testing.T) {
		// Skip if no test file
		testFile := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"
		if _, err := os.Stat(testFile); err != nil {
			t.Skip("Test video not available")
		}

		ctx := context.Background()
		
		req := plugins.TranscodeRequest{
			MediaID:    "test-stop",
			InputPath:  testFile,
			Container:  "mp4",
			VideoCodec: "libx264",
			AudioCodec: "aac",
		}

		handle, err := filePipeline.Transcode(ctx, req)
		require.NoError(t, err)

		// Let it run for a bit
		time.Sleep(2 * time.Second)

		// Stop it
		err = filePipeline.Stop(handle.SessionID)
		require.NoError(t, err)

		// Check status
		session, err := sessionStore.GetSession(handle.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", session.Status)
	})
}