package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamEncoder_AdaptiveSegmentIntegration(t *testing.T) {
	tempDir := t.TempDir()
	encoder := NewStreamEncoder(tempDir, 4)

	// Test adaptive segment functionality
	assert.NotNil(t, encoder.adaptiveSizer, "Adaptive sizer should be initialized")
	assert.False(t, encoder.adaptiveSegments, "Adaptive segments should be disabled by default")

	// Test enabling adaptive segments
	encoder.SetAdaptiveSegments(true)
	assert.True(t, encoder.adaptiveSegments, "Adaptive segments should be enabled")

	// Test optimization status
	status := encoder.GetOptimizationStatus()
	assert.True(t, status["adaptive_segments"].(bool), "Status should show adaptive segments enabled")
	assert.True(t, status["adaptive_sizer_available"].(bool), "Status should show adaptive sizer available")
}

func TestStreamEncoder_GenerateAdaptiveSegmentPlan(t *testing.T) {
	tempDir := t.TempDir()
	encoder := NewStreamEncoder(tempDir, 4)

	// Create a temporary test file
	testFile := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(testFile, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	// Test generating adaptive segment plan
	plan, err := encoder.GenerateAdaptiveSegmentPlan(ctx, testFile)
	assert.Error(t, err, "Should fail with dummy file")
	assert.Nil(t, plan, "Plan should be nil for dummy file")
}

func TestStreamEncoder_GetAdaptiveSegmentSummary(t *testing.T) {
	tempDir := t.TempDir()
	encoder := NewStreamEncoder(tempDir, 4)

	// Test with nil plan
	summary := encoder.GetAdaptiveSegmentSummary(nil)
	assert.Equal(t, "No segments generated", summary)

	// Create a mock plan
	plan := &AdaptiveSegmentPlan{
		TotalDuration:     10 * time.Second,
		SegmentCount:      3,
		AverageComplexity: 0.6,
		OptimizationScore: 0.85,
		Segments: []SegmentSizeDecision{
			{Duration: 3 * time.Second, KeyframeAligned: true},
			{Duration: 4 * time.Second, KeyframeAligned: true, SceneChangeDetected: true},
			{Duration: 3 * time.Second, KeyframeAligned: false},
		},
	}

	summary = encoder.GetAdaptiveSegmentSummary(plan)
	assert.Contains(t, summary, "Total Duration: 10s")
	assert.Contains(t, summary, "Segments: 3")
}

func TestStreamEncoder_AdaptiveSegmentConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	encoder := NewStreamEncoder(tempDir, 4)

	// Test default configuration
	status := encoder.GetOptimizationStatus()
	assert.False(t, status["adaptive_segments"].(bool), "Should be disabled by default")
	assert.True(t, status["fast_start_enabled"].(bool), "Fast start should be enabled")
	assert.True(t, status["seek_optimized"].(bool), "Seek optimization should be enabled")

	// Test enabling adaptive segments
	encoder.SetAdaptiveSegments(true)
	encoder.SetFastStartEnabled(false)
	encoder.SetSeekOptimized(false)

	status = encoder.GetOptimizationStatus()
	assert.True(t, status["adaptive_segments"].(bool), "Adaptive segments should be enabled")
	assert.False(t, status["fast_start_enabled"].(bool), "Fast start should be disabled")
	assert.False(t, status["seek_optimized"].(bool), "Seek optimization should be disabled")
}
