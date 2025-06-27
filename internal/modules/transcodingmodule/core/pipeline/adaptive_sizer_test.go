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

func TestAdaptiveSegmentSizer_NewAdaptiveSegmentSizer(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	assert.NotNil(t, sizer)
	assert.NotNil(t, sizer.fastStartOptimizer)
	assert.Equal(t, 4*time.Second, sizer.baseSegmentDuration)
	assert.Equal(t, 2*time.Second, sizer.minSegmentDuration)
	assert.Equal(t, 10*time.Second, sizer.maxSegmentDuration)
}

func TestAdaptiveSegmentSizer_GenerateAdaptiveSegmentPlan(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")

	// Create a dummy file for testing
	err := os.WriteFile(testFile, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	plan, err := sizer.GenerateAdaptiveSegmentPlan(ctx, testFile)

	// With a dummy file, keyframe analysis will fail
	assert.Error(t, err)
	assert.Nil(t, plan)
}

func TestAdaptiveSegmentSizer_CalculateAdaptiveSegmentDuration(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	tests := []struct {
		name                string
		complexityScores    []float64
		currentTime         time.Duration
		expectedMinDuration time.Duration
		expectedMaxDuration time.Duration
	}{
		{
			name:                "empty complexity scores",
			complexityScores:    []float64{},
			currentTime:         0,
			expectedMinDuration: 4 * time.Second,
			expectedMaxDuration: 4 * time.Second,
		},
		{
			name:                "high complexity",
			complexityScores:    []float64{0.8, 0.9, 0.7}, // High complexity
			currentTime:         0,
			expectedMinDuration: 2 * time.Second, // Should be shortened
			expectedMaxDuration: 4 * time.Second,
		},
		{
			name:                "low complexity",
			complexityScores:    []float64{0.1, 0.2, 0.1}, // Low complexity
			currentTime:         0,
			expectedMinDuration: 4 * time.Second,
			expectedMaxDuration: 10 * time.Second, // Should be lengthened
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := sizer.calculateAdaptiveSegmentDuration(tt.currentTime, tt.complexityScores)

			assert.True(t, duration >= tt.expectedMinDuration,
				"Duration %v should be >= %v", duration, tt.expectedMinDuration)
			assert.True(t, duration <= tt.expectedMaxDuration,
				"Duration %v should be <= %v", duration, tt.expectedMaxDuration)
		})
	}
}

func TestAdaptiveSegmentSizer_FindOptimalSegmentEnd(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	// Create test keyframes
	keyframes := []KeyframeInfo{
		{Index: 0, Timestamp: 0 * time.Second},
		{Index: 1, Timestamp: 2 * time.Second},
		{Index: 2, Timestamp: 4 * time.Second},
		{Index: 3, Timestamp: 6 * time.Second},
		{Index: 4, Timestamp: 8 * time.Second},
	}

	startTime := 1 * time.Second
	targetDuration := 4 * time.Second

	endTime, aligned := sizer.findOptimalSegmentEnd(keyframes, startTime, targetDuration)

	// Should find a nearby keyframe
	assert.True(t, endTime > startTime, "End time should be after start time")
	assert.True(t, endTime <= 8*time.Second, "End time should be within keyframe range")

	// Check if alignment is reasonable
	if aligned {
		// Should align to one of the keyframe timestamps
		found := false
		for _, kf := range keyframes {
			if kf.Timestamp == endTime {
				found = true
				break
			}
		}
		assert.True(t, found, "Aligned end time should match a keyframe timestamp")
	}
}

func TestAdaptiveSegmentSizer_GetComplexityForTimeRange(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	complexityScores := []float64{0.1, 0.3, 0.5, 0.7, 0.9}
	totalDuration := 10 * time.Second

	tests := []struct {
		name        string
		startTime   time.Duration
		endTime     time.Duration
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "first half",
			startTime:   0 * time.Second,
			endTime:     5 * time.Second,
			expectedMin: 0.1,
			expectedMax: 0.5,
		},
		{
			name:        "second half",
			startTime:   5 * time.Second,
			endTime:     10 * time.Second,
			expectedMin: 0.5,
			expectedMax: 0.9,
		},
		{
			name:        "middle segment",
			startTime:   2 * time.Second,
			endTime:     8 * time.Second,
			expectedMin: 0.1,
			expectedMax: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexity := sizer.getComplexityForTimeRange(complexityScores, tt.startTime, tt.endTime, totalDuration)

			assert.True(t, complexity >= tt.expectedMin,
				"Complexity %v should be >= %v", complexity, tt.expectedMin)
			assert.True(t, complexity <= tt.expectedMax,
				"Complexity %v should be <= %v", complexity, tt.expectedMax)
		})
	}
}

func TestAdaptiveSegmentSizer_DetectSceneChange(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	tests := []struct {
		name             string
		complexityScores []float64
		startTime        time.Duration
		endTime          time.Duration
		totalDuration    time.Duration
		expectChange     bool
	}{
		{
			name:             "no scene change",
			complexityScores: []float64{0.3, 0.3, 0.3, 0.3},
			startTime:        0 * time.Second,
			endTime:          4 * time.Second,
			totalDuration:    4 * time.Second,
			expectChange:     false,
		},
		{
			name:             "significant scene change",
			complexityScores: []float64{0.1, 0.1, 0.9, 0.9},
			startTime:        0 * time.Second,
			endTime:          4 * time.Second,
			totalDuration:    4 * time.Second,
			expectChange:     true,
		},
		{
			name:             "empty complexity scores",
			complexityScores: []float64{},
			startTime:        0 * time.Second,
			endTime:          4 * time.Second,
			totalDuration:    4 * time.Second,
			expectChange:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sceneChange := sizer.detectSceneChange(tt.complexityScores, tt.startTime, tt.endTime, tt.totalDuration)
			assert.Equal(t, tt.expectChange, sceneChange)
		})
	}
}

func TestAdaptiveSegmentSizer_DetermineSegmentReason(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	tests := []struct {
		name            string
		complexity      float64
		targetDuration  time.Duration
		actualDuration  time.Duration
		keyframeAligned bool
		sceneChange     bool
		expectedReason  string
	}{
		{
			name:            "high complexity keyframe aligned",
			complexity:      0.8,
			targetDuration:  4 * time.Second,
			actualDuration:  4 * time.Second,
			keyframeAligned: true,
			sceneChange:     false,
			expectedReason:  "high complexity, keyframe aligned",
		},
		{
			name:            "scene change detected",
			complexity:      0.3,
			targetDuration:  4 * time.Second,
			actualDuration:  4 * time.Second,
			keyframeAligned: true,
			sceneChange:     true,
			expectedReason:  "scene change, keyframe aligned",
		},
		{
			name:            "standard segment",
			complexity:      0.3,
			targetDuration:  4 * time.Second,
			actualDuration:  4 * time.Second,
			keyframeAligned: false,
			sceneChange:     false,
			expectedReason:  "standard segment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := sizer.determineSegmentReason(tt.complexity, tt.targetDuration, tt.actualDuration,
				tt.keyframeAligned, tt.sceneChange)
			assert.Contains(t, reason, tt.expectedReason)
		})
	}
}

func TestAdaptiveSegmentSizer_GenerateUniformSegmentPlan(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	// Create test keyframes
	keyframes := []KeyframeInfo{
		{Index: 0, Timestamp: 0 * time.Second},
		{Index: 1, Timestamp: 2 * time.Second},
		{Index: 2, Timestamp: 4 * time.Second},
		{Index: 3, Timestamp: 6 * time.Second},
		{Index: 4, Timestamp: 8 * time.Second},
	}

	plan := sizer.generateUniformSegmentPlan(keyframes)

	assert.NotNil(t, plan)
	assert.True(t, len(plan.Segments) > 0)
	assert.Equal(t, 8*time.Second, plan.TotalDuration)
	assert.Equal(t, 0.5, plan.AverageComplexity)

	// All segments should be keyframe aligned
	for _, segment := range plan.Segments {
		assert.True(t, segment.KeyframeAligned)
		assert.Equal(t, "uniform fallback", segment.Reason)
	}
}

func TestAdaptiveSegmentSizer_CalculateOptimizationScore(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	// Create a test plan
	plan := &AdaptiveSegmentPlan{
		Segments: []SegmentSizeDecision{
			{
				Duration:            4 * time.Second,
				ComplexityScore:     0.3,
				KeyframeAligned:     true,
				SceneChangeDetected: false,
			},
			{
				Duration:            4 * time.Second,
				ComplexityScore:     0.7,
				KeyframeAligned:     true,
				SceneChangeDetected: true,
			},
		},
	}

	score := sizer.calculateOptimizationScore(plan)

	assert.True(t, score >= 0.0, "Score should be non-negative")
	assert.True(t, score <= 1.0, "Score should not exceed 1.0")

	// Should get high score for keyframe alignment
	assert.True(t, score > 0.5, "Should get decent score for good alignment")
}

func TestAdaptiveSegmentSizer_GetSegmentPlanSummary(t *testing.T) {
	logger := hclog.NewNullLogger()
	optimizer := NewFastStartOptimizer(logger)
	sizer := NewAdaptiveSegmentSizer(logger, optimizer)

	// Test with nil plan
	summary := sizer.GetSegmentPlanSummary(nil)
	assert.Equal(t, "No segments generated", summary)

	// Test with valid plan
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

	summary = sizer.GetSegmentPlanSummary(plan)

	assert.Contains(t, summary, "Total Duration: 10s")
	assert.Contains(t, summary, "Segments: 3")
	assert.Contains(t, summary, "Average Complexity: 0.600")
	assert.Contains(t, summary, "Optimization Score: 0.850")
	assert.Contains(t, summary, "Scene Changes: 1")
}
