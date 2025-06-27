package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFastStartOptimizer_AnalyzeKeyframes(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")

	// Create a dummy file for testing
	err := os.WriteFile(testFile, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Test keyframe analysis (will return default values for dummy file)
	ctx := context.Background()
	keyframes, err := optimizer.AnalyzeKeyframes(ctx, testFile)

	// With a dummy file, ffprobe will fail but we should handle it gracefully
	assert.Error(t, err) // Expected to fail with dummy content
	assert.Empty(t, keyframes)
}

func TestFastStartOptimizer_GetOptimizationProfiles(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)

	// Test fast-start profile
	fastStartProfile := optimizer.GetFastStartProfile()
	assert.Equal(t, 30, fastStartProfile.GOPSize)
	assert.Equal(t, "main", fastStartProfile.Profile)
	assert.True(t, fastStartProfile.FastStart)

	// Test seek-optimized profile
	seekProfile := optimizer.GetSeekOptimizedProfile()
	assert.Equal(t, 60, seekProfile.GOPSize)
	assert.Equal(t, "high", seekProfile.Profile)
	assert.True(t, seekProfile.FastStart)
}

func TestFastStartOptimizer_OptimizeSegmentBoundaries(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)

	// Create test keyframes at regular intervals
	keyframes := []KeyframeInfo{
		{Index: 0, Timestamp: 0 * time.Second},
		{Index: 1, Timestamp: 2 * time.Second},
		{Index: 2, Timestamp: 4 * time.Second},
		{Index: 3, Timestamp: 6 * time.Second},
		{Index: 4, Timestamp: 8 * time.Second},
		{Index: 5, Timestamp: 10 * time.Second},
	}

	segmentDuration := 4 * time.Second

	boundaries := optimizer.OptimizeSegmentBoundaries(keyframes, segmentDuration)

	assert.NotEmpty(t, boundaries)

	// Check that boundaries are reasonable
	for _, boundary := range boundaries {
		assert.True(t, boundary >= 0, "Boundary should be non-negative")
		assert.True(t, boundary <= 10*time.Second, "Boundary should be within video duration")
	}
}

func TestFastStartOptimizer_AnalyzeSceneComplexity(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")

	// Create a dummy file for testing
	err := os.WriteFile(testFile, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Test scene complexity analysis
	ctx := context.Background()
	complexityScores, err := optimizer.AnalyzeSceneComplexity(ctx, testFile)

	// With a dummy file, ffprobe will fail but we should handle it gracefully
	assert.Error(t, err)              // Expected to fail with dummy content
	assert.Empty(t, complexityScores) // Should return empty slice
}

func TestFastStartOptimizer_GenerateSeekPoints(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)

	// Create test keyframes
	keyframes := []KeyframeInfo{
		{Index: 0, Timestamp: 0 * time.Second},
		{Index: 1, Timestamp: 2 * time.Second},
		{Index: 2, Timestamp: 4 * time.Second},
		{Index: 3, Timestamp: 6 * time.Second},
		{Index: 4, Timestamp: 8 * time.Second},
		{Index: 5, Timestamp: 10 * time.Second},
	}

	segmentDuration := 4 * time.Second

	seekPoints := optimizer.GenerateSeekPoints(keyframes, segmentDuration)

	assert.NotEmpty(t, seekPoints)

	// All seek points should be reasonable
	for _, point := range seekPoints {
		assert.True(t, point.Timestamp >= 0,
			"Seek point timestamp should be non-negative")
		assert.True(t, point.Timestamp <= 10*time.Second,
			"Seek point timestamp should be within video duration")
		assert.True(t, point.SegmentIndex >= 0,
			"Segment index should be non-negative")
	}
}
