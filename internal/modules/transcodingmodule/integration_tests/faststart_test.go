// Test fast-start optimization and keyframe alignment
package integration_tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/pipeline"
)

func TestFastStartOptimization(t *testing.T) {
	testVideo := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "faststart-test",
		Level: hclog.Debug,
	})

	t.Log("🚀 Testing fast-start optimization and keyframe alignment")

	// Create optimizer
	optimizer := pipeline.NewFastStartOptimizer(logger)

	// Test 1: Analyze keyframes in input video
	t.Log("🔍 Analyzing input video keyframes...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	keyframes, err := optimizer.AnalyzeKeyframes(ctx, testVideo)
	if err != nil {
		t.Fatalf("Failed to analyze keyframes: %v", err)
	}

	t.Logf("✅ Found %d keyframes in input video", len(keyframes))
	if len(keyframes) > 0 {
		t.Logf("   First keyframe: %v", keyframes[0].Timestamp)
		t.Logf("   Last keyframe: %v", keyframes[len(keyframes)-1].Timestamp)
	}

	// Test 2: Generate seek points
	t.Log("📍 Generating optimized seek points...")
	segmentDuration := 4 * time.Second
	seekPoints := optimizer.GenerateSeekPoints(keyframes, segmentDuration)

	t.Logf("✅ Generated %d seek points", len(seekPoints))
	for i, point := range seekPoints {
		if i < 3 { // Show first 3
			t.Logf("   Seek point %d: %v (accuracy: %.3fs)",
				i, point.Timestamp, point.Accuracy)
		}
	}

	// Test 3: Optimize segment boundaries
	t.Log("⚡ Optimizing segment boundaries...")
	boundaries := optimizer.OptimizeSegmentBoundaries(keyframes, segmentDuration)
	t.Logf("✅ Optimized to %d segment boundaries", len(boundaries))
	for i, boundary := range boundaries {
		if i < 3 { // Show first 3
			t.Logf("   Boundary %d: %v", i, boundary)
		}
	}

	// Test 4: Analyze scene complexity
	t.Log("🎬 Analyzing scene complexity...")
	complexity, err := optimizer.AnalyzeSceneComplexity(ctx, testVideo)
	if err != nil {
		t.Logf("⚠️ Scene complexity analysis failed: %v", err)
	} else {
		t.Logf("✅ Analyzed %d complexity scores", len(complexity))
		if len(complexity) > 0 {
			avg := calculateAverage(complexity)
			t.Logf("   Average complexity: %.3f", avg)
		}
	}

	// Test 5: Get optimization profiles
	t.Log("⚙️ Testing optimization profiles...")

	fastStartProfile := optimizer.GetFastStartProfile()
	t.Logf("Fast-start profile: GOP=%d, Profile=%s, Preset=%s",
		fastStartProfile.GOPSize, fastStartProfile.Profile, fastStartProfile.Preset)

	seekProfile := optimizer.GetSeekOptimizedProfile()
	t.Logf("Seek-optimized profile: GOP=%d, Profile=%s, Preset=%s",
		seekProfile.GOPSize, seekProfile.Profile, seekProfile.Preset)

	// Test 6: Generate optimized encoding args
	t.Log("🔧 Testing optimized encoding arguments...")

	fastStartArgs := optimizer.GetOptimizedEncodingArgs(fastStartProfile)
	t.Logf("Fast-start args (%d): %v", len(fastStartArgs), fastStartArgs[:min(len(fastStartArgs), 10)])

	seekArgs := optimizer.GetOptimizedEncodingArgs(seekProfile)
	t.Logf("Seek-optimized args (%d): %v", len(seekArgs), seekArgs[:min(len(seekArgs), 10)])

	// Validate results
	if len(keyframes) == 0 {
		t.Error("❌ No keyframes found in input video")
	}

	if len(seekPoints) == 0 {
		t.Error("❌ No seek points generated")
	}

	if len(fastStartArgs) == 0 {
		t.Error("❌ No fast-start args generated")
	}

	t.Log("✅ Fast-start optimization test completed")
}

func TestStreamEncoderOptimization(t *testing.T) {
	testVideo := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "optimized_encoding")

	// Create logger
	_ = hclog.New(&hclog.LoggerOptions{
		Name:  "encoder-optimization-test",
		Level: hclog.Info,
	})

	t.Log("🎯 Testing stream encoder with fast-start optimization")

	// Create encoder with optimization
	encoder := pipeline.NewStreamEncoder(outputDir, 3)

	// Test optimization settings
	status := encoder.GetOptimizationStatus()
	t.Logf("Initial optimization status: %+v", status)

	// Enable different optimization modes
	encoder.SetFastStartEnabled(true)
	encoder.SetSeekOptimized(true)

	var segmentCount int
	var optimizationReports []*pipeline.OptimizationReport

	encoder.SetCallbacks(
		func(segmentPath string, segmentIndex int) {
			segmentCount++
			t.Logf("✅ Optimized segment %d ready: %s", segmentIndex, filepath.Base(segmentPath))

			// Validate optimization on first few segments
			if segmentIndex < 2 {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				if report, err := encoder.ValidateSegmentOptimization(ctx, segmentPath); err == nil {
					optimizationReports = append(optimizationReports, report)
					t.Logf("   Optimization score: %.2f", report.OptimizationScore)
					t.Logf("   Fast-start: %v", report.HasFastStart)
					t.Logf("   Keyframes: %d", report.KeyframeCount)
				} else {
					t.Logf("   Validation failed: %v", err)
				}
			}
		},
		func(err error) {
			t.Logf("❌ Encoder error: %v", err)
		},
	)

	// Test input analysis
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Log("🔍 Analyzing input complexity...")
	if complexity, err := encoder.AnalyzeInputComplexity(ctx, testVideo); err == nil {
		t.Logf("   Complexity scores: %d", len(complexity))
	} else {
		t.Logf("   Analysis failed: %v", err)
	}

	t.Log("⚡ Optimizing segment boundaries...")
	if boundaries, err := encoder.OptimizeSegmentBoundaries(ctx, testVideo); err == nil {
		t.Logf("   Optimized boundaries: %d", len(boundaries))
	} else {
		t.Logf("   Boundary optimization failed: %v", err)
	}

	// Create optimized profile
	profiles := []pipeline.EncodingProfile{
		{
			Name:         "optimized",
			Width:        640,
			Height:       360,
			VideoBitrate: 800,
			Quality:      26,
		},
	}

	// Start optimized encoding
	t.Log("🚀 Starting optimized encoding...")
	err := encoder.StartEncoding(ctx, testVideo, profiles)
	if err != nil {
		t.Fatalf("Failed to start optimized encoding: %v", err)
	}

	// Wait for segments
	time.Sleep(8 * time.Second)
	encoder.StopEncoding()

	// Analyze results
	t.Logf("\n=== OPTIMIZATION RESULTS ===")
	t.Logf("Segments produced: %d", segmentCount)
	t.Logf("Optimization reports: %d", len(optimizationReports))

	finalStatus := encoder.GetOptimizationStatus()
	t.Logf("Final optimization status: %+v", finalStatus)

	// Calculate average optimization score
	if len(optimizationReports) > 0 {
		totalScore := 0.0
		for _, report := range optimizationReports {
			totalScore += report.OptimizationScore
		}
		avgScore := totalScore / float64(len(optimizationReports))
		t.Logf("Average optimization score: %.2f", avgScore)

		if avgScore > 0.5 {
			t.Logf("✅ Good optimization score: %.2f", avgScore)
		} else {
			t.Logf("⚠️ Low optimization score: %.2f", avgScore)
		}
	}

	if segmentCount > 0 {
		t.Logf("✅ Optimized encoding successful: %d segments", segmentCount)
	} else {
		t.Error("❌ No segments produced with optimization")
	}

	t.Log("✅ Stream encoder optimization test completed")
}

func TestKeyframeAlignment(t *testing.T) {
	testVideo := "/home/fictional/Projects/viewra/viewra-data/test-video.mp4"

	t.Log("🎯 Testing keyframe alignment for seek optimization")

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "keyframe-test",
		Level: hclog.Debug,
	})

	optimizer := pipeline.NewFastStartOptimizer(logger)

	// Analyze keyframes
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	keyframes, err := optimizer.AnalyzeKeyframes(ctx, testVideo)
	if err != nil {
		t.Fatalf("Failed to analyze keyframes: %v", err)
	}

	if len(keyframes) == 0 {
		t.Fatal("No keyframes found")
	}

	t.Logf("Analyzing %d keyframes", len(keyframes))

	// Test different segment durations
	segmentDurations := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		6 * time.Second,
	}

	for _, duration := range segmentDurations {
		t.Logf("\n--- Testing %v segments ---", duration)

		// Generate seek points
		seekPoints := optimizer.GenerateSeekPoints(keyframes, duration)
		t.Logf("Seek points: %d", len(seekPoints))

		// Calculate accuracy statistics
		if len(seekPoints) > 0 {
			totalAccuracy := 0.0
			maxAccuracy := 0.0

			for _, point := range seekPoints {
				totalAccuracy += point.Accuracy
				if point.Accuracy > maxAccuracy {
					maxAccuracy = point.Accuracy
				}
			}

			avgAccuracy := totalAccuracy / float64(len(seekPoints))
			t.Logf("   Average accuracy: %.3fs", avgAccuracy)
			t.Logf("   Max deviation: %.3fs", maxAccuracy)

			// Good accuracy is within 100ms
			if avgAccuracy <= 0.1 {
				t.Logf("   ✅ Excellent seek accuracy")
			} else if avgAccuracy <= 0.5 {
				t.Logf("   ✅ Good seek accuracy")
			} else {
				t.Logf("   ⚠️ Poor seek accuracy")
			}
		}

		// Optimize boundaries
		boundaries := optimizer.OptimizeSegmentBoundaries(keyframes, duration)
		t.Logf("Optimized boundaries: %d", len(boundaries))

		// Check boundary alignment
		alignedCount := 0
		for _, boundary := range boundaries {
			// Check if boundary aligns with a keyframe (within 100ms)
			for _, kf := range keyframes {
				if abs64(int64(boundary)-int64(kf.Timestamp)) <= int64(100*time.Millisecond) {
					alignedCount++
					break
				}
			}
		}

		alignmentRatio := float64(alignedCount) / float64(len(boundaries))
		t.Logf("   Keyframe alignment: %.1f%% (%d/%d)",
			alignmentRatio*100, alignedCount, len(boundaries))

		if alignmentRatio >= 0.8 {
			t.Logf("   ✅ Excellent keyframe alignment")
		} else if alignmentRatio >= 0.6 {
			t.Logf("   ✅ Good keyframe alignment")
		} else {
			t.Logf("   ⚠️ Poor keyframe alignment")
		}
	}

	t.Log("✅ Keyframe alignment test completed")
}

// Helper functions
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
