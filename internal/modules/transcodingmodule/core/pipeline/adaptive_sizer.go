// Package pipeline provides adaptive segment sizing based on scene complexity
package pipeline

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/go-hclog"
)

// AdaptiveSegmentSizer handles intelligent segment duration based on content analysis
type AdaptiveSegmentSizer struct {
	logger hclog.Logger

	// Configuration
	baseSegmentDuration time.Duration // Base segment duration (e.g., 4 seconds)
	minSegmentDuration  time.Duration // Minimum allowed segment duration
	maxSegmentDuration  time.Duration // Maximum allowed segment duration

	// Adaptive sizing parameters
	complexityThreshold  float64 // Complexity threshold for size adjustment
	complexityMultiplier float64 // How much complexity affects segment size
	sceneChangeThreshold float64 // Scene change detection sensitivity

	// Integration with fast-start optimizer
	fastStartOptimizer *FastStartOptimizer
}

// SegmentSizeDecision represents a segment sizing decision
type SegmentSizeDecision struct {
	StartTime           time.Duration `json:"start_time"`
	EndTime             time.Duration `json:"end_time"`
	Duration            time.Duration `json:"duration"`
	ComplexityScore     float64       `json:"complexity_score"`
	SceneChangeDetected bool          `json:"scene_change_detected"`
	KeyframeAligned     bool          `json:"keyframe_aligned"`
	Reason              string        `json:"reason"`
}

// AdaptiveSegmentPlan contains the complete segmentation strategy
type AdaptiveSegmentPlan struct {
	InputPath         string                `json:"input_path"`
	TotalDuration     time.Duration         `json:"total_duration"`
	SegmentCount      int                   `json:"segment_count"`
	Segments          []SegmentSizeDecision `json:"segments"`
	AverageComplexity float64               `json:"average_complexity"`
	OptimizationScore float64               `json:"optimization_score"`
	GeneratedAt       time.Time             `json:"generated_at"`
}

// NewAdaptiveSegmentSizer creates a new adaptive segment sizer
func NewAdaptiveSegmentSizer(logger hclog.Logger, fastStartOptimizer *FastStartOptimizer) *AdaptiveSegmentSizer {
	return &AdaptiveSegmentSizer{
		logger:              logger,
		baseSegmentDuration: 4 * time.Second,  // Netflix-style 4-second segments
		minSegmentDuration:  2 * time.Second,  // Minimum 2 seconds for compatibility
		maxSegmentDuration:  10 * time.Second, // Maximum 10 seconds for startup time

		complexityThreshold:  0.5, // 50% complexity threshold
		complexityMultiplier: 0.3, // 30% adjustment factor
		sceneChangeThreshold: 0.4, // 40% scene change threshold

		fastStartOptimizer: fastStartOptimizer,
	}
}

// GenerateAdaptiveSegmentPlan creates an optimized segmentation strategy
func (ass *AdaptiveSegmentSizer) GenerateAdaptiveSegmentPlan(ctx context.Context, inputPath string) (*AdaptiveSegmentPlan, error) {
	ass.logger.Debug("Generating adaptive segment plan", "input", inputPath)

	// Analyze keyframes for precise timing
	keyframes, err := ass.fastStartOptimizer.AnalyzeKeyframes(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze keyframes: %w", err)
	}

	if len(keyframes) == 0 {
		return nil, fmt.Errorf("no keyframes found in video")
	}

	// Analyze scene complexity
	complexityScores, err := ass.fastStartOptimizer.AnalyzeSceneComplexity(ctx, inputPath)
	if err != nil {
		ass.logger.Warn("Failed to analyze scene complexity, using defaults", "error", err)
		// Fall back to uniform segmentation
		return ass.generateUniformSegmentPlan(keyframes), nil
	}

	// Generate adaptive segmentation
	plan := ass.createAdaptivePlan(inputPath, keyframes, complexityScores)

	ass.logger.Debug("Generated adaptive segment plan",
		"segments", len(plan.Segments),
		"total_duration", plan.TotalDuration,
		"avg_complexity", plan.AverageComplexity,
		"optimization_score", plan.OptimizationScore,
	)

	return plan, nil
}

// createAdaptivePlan generates segments based on complexity and keyframe alignment
func (ass *AdaptiveSegmentSizer) createAdaptivePlan(inputPath string, keyframes []KeyframeInfo, complexityScores []float64) *AdaptiveSegmentPlan {
	totalDuration := keyframes[len(keyframes)-1].Timestamp

	plan := &AdaptiveSegmentPlan{
		InputPath:     inputPath,
		TotalDuration: totalDuration,
		Segments:      []SegmentSizeDecision{},
		GeneratedAt:   time.Now(),
	}

	// Calculate average complexity for reference
	plan.AverageComplexity = ass.calculateAverageComplexity(complexityScores)

	currentTime := time.Duration(0)
	segmentIndex := 0

	for currentTime < totalDuration {
		// Determine adaptive segment duration
		adaptiveDuration := ass.calculateAdaptiveSegmentDuration(currentTime, complexityScores)

		// Find optimal end time aligned to keyframes
		endTime, keyframeAligned := ass.findOptimalSegmentEnd(keyframes, currentTime, adaptiveDuration)

		// Ensure we don't exceed total duration
		if endTime > totalDuration {
			endTime = totalDuration
		}

		actualDuration := endTime - currentTime

		// Get complexity score for this segment
		complexityScore := ass.getComplexityForTimeRange(complexityScores, currentTime, endTime, totalDuration)

		// Detect scene changes
		sceneChangeDetected := ass.detectSceneChange(complexityScores, currentTime, endTime, totalDuration)

		// Determine the reason for this segment size
		reason := ass.determineSegmentReason(complexityScore, adaptiveDuration, actualDuration, keyframeAligned, sceneChangeDetected)

		segment := SegmentSizeDecision{
			StartTime:           currentTime,
			EndTime:             endTime,
			Duration:            actualDuration,
			ComplexityScore:     complexityScore,
			SceneChangeDetected: sceneChangeDetected,
			KeyframeAligned:     keyframeAligned,
			Reason:              reason,
		}

		plan.Segments = append(plan.Segments, segment)

		currentTime = endTime
		segmentIndex++

		// Safety check to prevent infinite loops
		if segmentIndex > 1000 {
			ass.logger.Warn("Segment count exceeded safety limit, terminating")
			break
		}
	}

	plan.SegmentCount = len(plan.Segments)
	plan.OptimizationScore = ass.calculateOptimizationScore(plan)

	return plan
}

// calculateAdaptiveSegmentDuration determines segment duration based on complexity
func (ass *AdaptiveSegmentSizer) calculateAdaptiveSegmentDuration(currentTime time.Duration, complexityScores []float64) time.Duration {
	if len(complexityScores) == 0 {
		return ass.baseSegmentDuration
	}

	// Get complexity score for current time
	timeRatio := float64(currentTime) / float64(len(complexityScores)) / float64(time.Second)
	scoreIndex := int(timeRatio)

	if scoreIndex >= len(complexityScores) {
		scoreIndex = len(complexityScores) - 1
	}

	complexity := complexityScores[scoreIndex]

	// Adjust segment duration based on complexity
	var adjustmentFactor float64
	if complexity > ass.complexityThreshold {
		// High complexity: shorter segments for better quality
		adjustmentFactor = 1.0 - (complexity-ass.complexityThreshold)*ass.complexityMultiplier
	} else {
		// Low complexity: longer segments for efficiency
		adjustmentFactor = 1.0 + (ass.complexityThreshold-complexity)*ass.complexityMultiplier*0.5
	}

	// Apply adjustment
	adjustedDuration := time.Duration(float64(ass.baseSegmentDuration) * adjustmentFactor)

	// Clamp to min/max bounds
	if adjustedDuration < ass.minSegmentDuration {
		adjustedDuration = ass.minSegmentDuration
	} else if adjustedDuration > ass.maxSegmentDuration {
		adjustedDuration = ass.maxSegmentDuration
	}

	return adjustedDuration
}

// findOptimalSegmentEnd finds the best segment end time aligned to keyframes
func (ass *AdaptiveSegmentSizer) findOptimalSegmentEnd(keyframes []KeyframeInfo, startTime, targetDuration time.Duration) (time.Duration, bool) {
	targetEndTime := startTime + targetDuration

	// Find keyframes around the target end time
	var bestKeyframe *KeyframeInfo
	bestDistance := time.Duration(math.MaxInt64)

	for i := range keyframes {
		kf := &keyframes[i]

		// Only consider keyframes after start time
		if kf.Timestamp <= startTime {
			continue
		}

		distance := time.Duration(math.Abs(float64(kf.Timestamp - targetEndTime)))

		// Prefer keyframes that are close to target
		if distance < bestDistance {
			bestDistance = distance
			bestKeyframe = kf
		}

		// If we've gone past reasonable alignment distance, stop
		if kf.Timestamp > targetEndTime+time.Second {
			break
		}
	}

	if bestKeyframe != nil {
		// Check if alignment is reasonable (within 50% of base segment duration)
		maxDeviation := ass.baseSegmentDuration / 2
		if bestDistance <= maxDeviation {
			return bestKeyframe.Timestamp, true
		}
	}

	// Fall back to target time if no suitable keyframe
	return targetEndTime, false
}

// getComplexityForTimeRange calculates average complexity for a time range
func (ass *AdaptiveSegmentSizer) getComplexityForTimeRange(complexityScores []float64, startTime, endTime, totalDuration time.Duration) float64 {
	if len(complexityScores) == 0 {
		return 0.5 // Default complexity
	}

	// Map time range to complexity score indices
	totalSeconds := totalDuration.Seconds()
	startRatio := startTime.Seconds() / totalSeconds
	endRatio := endTime.Seconds() / totalSeconds

	startIndex := int(startRatio * float64(len(complexityScores)))
	endIndex := int(endRatio * float64(len(complexityScores)))

	// Clamp indices
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex >= len(complexityScores) {
		endIndex = len(complexityScores) - 1
	}
	if startIndex > endIndex {
		startIndex = endIndex
	}

	// Calculate average complexity for the range
	var sum float64
	count := 0
	for i := startIndex; i <= endIndex; i++ {
		sum += complexityScores[i]
		count++
	}

	if count == 0 {
		return 0.5
	}

	return sum / float64(count)
}

// detectSceneChange detects if there's a significant scene change in the segment
func (ass *AdaptiveSegmentSizer) detectSceneChange(complexityScores []float64, startTime, endTime, totalDuration time.Duration) bool {
	if len(complexityScores) < 2 {
		return false
	}

	// Get complexity values for the segment
	startComplexity := ass.getComplexityForTimeRange(complexityScores, startTime, startTime+time.Second, totalDuration)
	endComplexity := ass.getComplexityForTimeRange(complexityScores, endTime-time.Second, endTime, totalDuration)

	// Check for significant complexity change
	complexityChange := math.Abs(endComplexity - startComplexity)

	return complexityChange > ass.sceneChangeThreshold
}

// determineSegmentReason explains why a particular segment size was chosen
func (ass *AdaptiveSegmentSizer) determineSegmentReason(complexity float64, targetDuration, actualDuration time.Duration, keyframeAligned, sceneChange bool) string {
	reasons := []string{}

	if complexity > ass.complexityThreshold {
		reasons = append(reasons, "high complexity")
	} else if complexity < ass.complexityThreshold*0.5 {
		reasons = append(reasons, "low complexity")
	}

	if sceneChange {
		reasons = append(reasons, "scene change")
	}

	if keyframeAligned {
		reasons = append(reasons, "keyframe aligned")
	}

	deviation := actualDuration - targetDuration
	if deviation > time.Second {
		reasons = append(reasons, "extended for alignment")
	} else if deviation < -time.Second {
		reasons = append(reasons, "shortened for alignment")
	}

	if len(reasons) == 0 {
		return "standard segment"
	}

	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += ", " + reasons[i]
	}

	return result
}

// generateUniformSegmentPlan creates a fallback uniform segmentation
func (ass *AdaptiveSegmentSizer) generateUniformSegmentPlan(keyframes []KeyframeInfo) *AdaptiveSegmentPlan {
	totalDuration := keyframes[len(keyframes)-1].Timestamp

	plan := &AdaptiveSegmentPlan{
		InputPath:         "unknown",
		TotalDuration:     totalDuration,
		Segments:          []SegmentSizeDecision{},
		AverageComplexity: 0.5, // Default complexity
		GeneratedAt:       time.Now(),
	}

	// Use fast-start optimizer to align segments with keyframes
	boundaries := ass.fastStartOptimizer.OptimizeSegmentBoundaries(keyframes, ass.baseSegmentDuration)

	previousTime := time.Duration(0)
	for _, boundary := range boundaries {
		duration := boundary - previousTime

		segment := SegmentSizeDecision{
			StartTime:           previousTime,
			EndTime:             boundary,
			Duration:            duration,
			ComplexityScore:     0.5, // Default
			SceneChangeDetected: false,
			KeyframeAligned:     true,
			Reason:              "uniform fallback",
		}

		plan.Segments = append(plan.Segments, segment)
		previousTime = boundary
	}

	plan.SegmentCount = len(plan.Segments)
	plan.OptimizationScore = 0.7 // Reasonable score for uniform segmentation

	return plan
}

// calculateOptimizationScore evaluates the quality of the segmentation plan
func (ass *AdaptiveSegmentSizer) calculateOptimizationScore(plan *AdaptiveSegmentPlan) float64 {
	if len(plan.Segments) == 0 {
		return 0.0
	}

	score := 0.0

	// Keyframe alignment score (40% weight)
	alignedCount := 0
	for _, segment := range plan.Segments {
		if segment.KeyframeAligned {
			alignedCount++
		}
	}
	alignmentScore := float64(alignedCount) / float64(len(plan.Segments))
	score += 0.4 * alignmentScore

	// Duration variance score (30% weight)
	// Lower variance is better for streaming consistency
	durations := make([]float64, len(plan.Segments))
	var avgDuration float64
	for i, segment := range plan.Segments {
		durations[i] = segment.Duration.Seconds()
		avgDuration += durations[i]
	}
	avgDuration /= float64(len(durations))

	variance := 0.0
	for _, duration := range durations {
		diff := duration - avgDuration
		variance += diff * diff
	}
	variance /= float64(len(durations))

	// Normalize variance score (lower variance = higher score)
	varianceScore := 1.0 / (1.0 + variance)
	score += 0.3 * varianceScore

	// Complexity adaptation score (20% weight)
	// Check if high complexity areas have shorter segments
	adaptationScore := 0.0
	for _, segment := range plan.Segments {
		if segment.ComplexityScore > ass.complexityThreshold && segment.Duration < ass.baseSegmentDuration {
			adaptationScore += 1.0
		} else if segment.ComplexityScore < ass.complexityThreshold && segment.Duration >= ass.baseSegmentDuration {
			adaptationScore += 1.0
		}
	}
	adaptationScore /= float64(len(plan.Segments))
	score += 0.2 * adaptationScore

	// Scene change handling score (10% weight)
	sceneChangeScore := 0.0
	for _, segment := range plan.Segments {
		if segment.SceneChangeDetected {
			sceneChangeScore += 1.0
		}
	}
	if sceneChangeScore > 0 {
		sceneChangeScore = sceneChangeScore / float64(len(plan.Segments))
		score += 0.1 * sceneChangeScore
	} else {
		score += 0.1 // No scene changes is also good
	}

	return score
}

// calculateAverageComplexity calculates the average complexity across all scores
func (ass *AdaptiveSegmentSizer) calculateAverageComplexity(complexityScores []float64) float64 {
	if len(complexityScores) == 0 {
		return 0.5
	}

	var sum float64
	for _, score := range complexityScores {
		sum += score
	}

	return sum / float64(len(complexityScores))
}

// GetSegmentPlanSummary returns a human-readable summary of the plan
func (ass *AdaptiveSegmentSizer) GetSegmentPlanSummary(plan *AdaptiveSegmentPlan) string {
	if plan == nil || len(plan.Segments) == 0 {
		return "No segments generated"
	}

	// Calculate statistics
	var totalDuration, minDuration, maxDuration time.Duration
	minDuration = time.Duration(math.MaxInt64)
	keyframeAlignedCount := 0
	sceneChangeCount := 0

	for _, segment := range plan.Segments {
		totalDuration += segment.Duration

		if segment.Duration < minDuration {
			minDuration = segment.Duration
		}
		if segment.Duration > maxDuration {
			maxDuration = segment.Duration
		}

		if segment.KeyframeAligned {
			keyframeAlignedCount++
		}
		if segment.SceneChangeDetected {
			sceneChangeCount++
		}
	}

	avgDuration := totalDuration / time.Duration(len(plan.Segments))

	return fmt.Sprintf(
		"Adaptive Segmentation Plan:\n"+
			"  Total Duration: %v\n"+
			"  Segments: %d\n"+
			"  Average Duration: %v (min: %v, max: %v)\n"+
			"  Keyframe Aligned: %d/%d (%.1f%%)\n"+
			"  Scene Changes: %d\n"+
			"  Average Complexity: %.3f\n"+
			"  Optimization Score: %.3f",
		plan.TotalDuration,
		plan.SegmentCount,
		avgDuration, minDuration, maxDuration,
		keyframeAlignedCount, len(plan.Segments), float64(keyframeAlignedCount)/float64(len(plan.Segments))*100,
		sceneChangeCount,
		plan.AverageComplexity,
		plan.OptimizationScore,
	)
}
