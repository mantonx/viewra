// Package pipeline provides fast-start optimization with keyframe alignment for quick seek
package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// FastStartOptimizer handles keyframe alignment and seek optimization
type FastStartOptimizer struct {
	logger      hclog.Logger
	ffmpegPath  string
	ffprobePath string

	// Optimization settings
	keyframeInterval int     // GOP size (e.g., 60 frames = 2s at 30fps)
	seekAccuracy     float64 // Seek accuracy in seconds (0.1 = 100ms)
	fastStartEnabled bool    // Enable MP4 fast-start optimization
	sceneThreshold   float64 // Scene change detection threshold (0.0-1.0)
}

// KeyframeInfo represents keyframe timing information
type KeyframeInfo struct {
	Index      int           `json:"index"`
	Timestamp  time.Duration `json:"timestamp"`
	Size       int64         `json:"size"`
	Position   int64         `json:"position"`
	SceneScore float64       `json:"scene_score"` // Scene complexity score
}

// SeekPoint represents an optimized seek point
type SeekPoint struct {
	Timestamp     time.Duration `json:"timestamp"`
	SegmentIndex  int           `json:"segment_index"`
	KeyframeIndex int           `json:"keyframe_index"`
	ByteOffset    int64         `json:"byte_offset"`
	Accuracy      float64       `json:"accuracy"`
}

// OptimizationProfile defines encoding parameters for fast seeking
type OptimizationProfile struct {
	// Keyframe settings
	GOPSize          int  `json:"gop_size"`          // Group of Pictures size
	KeyframeInterval int  `json:"keyframe_interval"` // Forced keyframe interval
	SceneDetection   bool `json:"scene_detection"`   // Enable scene change detection

	// Fast-start settings
	MovAtom      bool `json:"mov_atom"`      // Move moov atom to beginning
	FragmentHint bool `json:"fragment_hint"` // Add fragmentation hints
	FastStart    bool `json:"fast_start"`    // Enable fast-start optimization

	// Seek optimization
	SeekPreroll      int `json:"seek_preroll"`      // Frames before keyframe for smooth seek
	IndexGranularity int `json:"index_granularity"` // Index entry frequency

	// Quality vs Speed trade-offs
	Profile string `json:"profile"` // H.264 profile (baseline, main, high)
	Preset  string `json:"preset"`  // Encoding preset (ultrafast, fast, medium)
	Tune    string `json:"tune"`    // Encoding tune (fastdecode, zerolatency)
}

// NewFastStartOptimizer creates a new fast-start optimizer
func NewFastStartOptimizer(logger hclog.Logger) *FastStartOptimizer {
	return &FastStartOptimizer{
		logger:           logger,
		ffmpegPath:       "ffmpeg",
		ffprobePath:      "ffprobe",
		keyframeInterval: 60,  // 2 seconds at 30fps
		seekAccuracy:     0.1, // 100ms accuracy
		fastStartEnabled: true,
		sceneThreshold:   0.3, // 30% scene change threshold
	}
}

// GetOptimizedEncodingArgs returns FFmpeg args optimized for fast seeking
func (fso *FastStartOptimizer) GetOptimizedEncodingArgs(profile OptimizationProfile) []string {
	var args []string

	// Video codec settings optimized for seeking
	args = append(args,
		"-c:v", "libx264",
		"-profile:v", profile.Profile,
		"-preset", profile.Preset,
	)

	// Tune for fast decode if specified
	if profile.Tune != "" {
		args = append(args, "-tune", profile.Tune)
	}

	// Keyframe settings for precise seeking
	args = append(args,
		"-g", fmt.Sprintf("%d", profile.GOPSize), // GOP size
		"-keyint_min", fmt.Sprintf("%d", profile.GOPSize/2), // Minimum keyframe interval
		"-forced-idr", "1", // Force IDR frames at keyframes
	)

	// Scene change detection
	if profile.SceneDetection {
		args = append(args,
			"-sc_threshold", fmt.Sprintf("%.2f", fso.sceneThreshold),
		)
	} else {
		args = append(args,
			"-sc_threshold", "0", // Disable scene change detection for consistent GOP
		)
	}

	// Fast-start optimization for MP4
	if profile.FastStart {
		args = append(args,
			"-movflags", "+faststart+frag_keyframe+empty_moov+default_base_moof",
		)
	}

	// Fragmentation for better seeking
	if profile.FragmentHint {
		args = append(args,
			"-frag_duration", "2000000", // 2-second fragments
			"-frag_size", "1048576", // 1MB fragment size
		)
	}

	// Index optimization
	args = append(args,
		"-write_tmcd", "0", // Disable timecode track
		"-map_metadata", "-1", // Strip metadata for smaller headers
	)

	fso.logger.Debug("Generated fast-start encoding args",
		"gop_size", profile.GOPSize,
		"profile", profile.Profile,
		"preset", profile.Preset,
		"fast_start", profile.FastStart,
	)

	return args
}

// AnalyzeKeyframes extracts keyframe information from a video file
func (fso *FastStartOptimizer) AnalyzeKeyframes(ctx context.Context, inputPath string) ([]KeyframeInfo, error) {
	fso.logger.Debug("Analyzing keyframes", "input", inputPath)

	// Use ffprobe to extract keyframe information
	args := []string{
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "frame=key_frame,pkt_pts_time,pkt_size,pkt_pos",
		"-of", "csv=print_section=0",
		"-print_format", "csv",
		inputPath,
	}

	cmd := exec.CommandContext(ctx, fso.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to analyze keyframes: %w", err)
	}

	return fso.parseKeyframeOutput(string(output))
}

// parseKeyframeOutput parses ffprobe keyframe output
func (fso *FastStartOptimizer) parseKeyframeOutput(output string) ([]KeyframeInfo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var keyframes []KeyframeInfo
	keyframeIndex := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 4 {
			continue
		}

		// Parse: key_frame,pkt_pts_time,pkt_size,pkt_pos
		isKeyframe := fields[0] == "1"
		if !isKeyframe {
			continue
		}

		timestamp, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}

		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			size = 0
		}

		position, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			position = 0
		}

		keyframe := KeyframeInfo{
			Index:     keyframeIndex,
			Timestamp: time.Duration(timestamp * float64(time.Second)),
			Size:      size,
			Position:  position,
		}

		keyframes = append(keyframes, keyframe)
		keyframeIndex++
	}

	fso.logger.Debug("Extracted keyframes",
		"count", len(keyframes),
		"duration", keyframes[len(keyframes)-1].Timestamp,
	)

	return keyframes, nil
}

// GenerateSeekPoints creates optimized seek points for a video
func (fso *FastStartOptimizer) GenerateSeekPoints(keyframes []KeyframeInfo, segmentDuration time.Duration) []SeekPoint {
	var seekPoints []SeekPoint

	if len(keyframes) == 0 {
		return seekPoints
	}

	// Calculate segment boundaries
	segmentDurationSeconds := segmentDuration.Seconds()
	totalDuration := keyframes[len(keyframes)-1].Timestamp
	segmentCount := int(totalDuration.Seconds()/segmentDurationSeconds) + 1

	fso.logger.Debug("Generating seek points",
		"keyframes", len(keyframes),
		"segments", segmentCount,
		"segment_duration", segmentDuration,
	)

	// Generate seek points aligned to keyframes
	for segmentIndex := 0; segmentIndex < segmentCount; segmentIndex++ {
		segmentStart := time.Duration(float64(segmentIndex) * segmentDurationSeconds * float64(time.Second))

		// Find closest keyframe to segment start
		bestKeyframe := fso.findClosestKeyframe(keyframes, segmentStart)
		if bestKeyframe == nil {
			continue
		}

		// Calculate seek accuracy
		accuracy := float64(abs64(int64(segmentStart)-int64(bestKeyframe.Timestamp))) / float64(time.Second)

		seekPoint := SeekPoint{
			Timestamp:     bestKeyframe.Timestamp,
			SegmentIndex:  segmentIndex,
			KeyframeIndex: bestKeyframe.Index,
			ByteOffset:    bestKeyframe.Position,
			Accuracy:      accuracy,
		}

		seekPoints = append(seekPoints, seekPoint)
	}

	fso.logger.Debug("Generated seek points",
		"count", len(seekPoints),
		"avg_accuracy", fso.calculateAverageAccuracy(seekPoints),
	)

	return seekPoints
}

// findClosestKeyframe finds the keyframe closest to the target timestamp
func (fso *FastStartOptimizer) findClosestKeyframe(keyframes []KeyframeInfo, targetTime time.Duration) *KeyframeInfo {
	if len(keyframes) == 0 {
		return nil
	}

	bestKeyframe := &keyframes[0]
	bestDistance := abs64(int64(targetTime) - int64(bestKeyframe.Timestamp))

	for i := 1; i < len(keyframes); i++ {
		distance := abs64(int64(targetTime) - int64(keyframes[i].Timestamp))
		if distance < bestDistance {
			bestDistance = distance
			bestKeyframe = &keyframes[i]
		}

		// If we've passed the target, the previous keyframe is closest
		if keyframes[i].Timestamp > targetTime {
			break
		}
	}

	return bestKeyframe
}

// OptimizeSegmentBoundaries adjusts segment boundaries to align with keyframes
func (fso *FastStartOptimizer) OptimizeSegmentBoundaries(keyframes []KeyframeInfo, targetSegmentDuration time.Duration) []time.Duration {
	var optimizedBoundaries []time.Duration

	if len(keyframes) == 0 {
		return optimizedBoundaries
	}

	targetDurationSeconds := targetSegmentDuration.Seconds()
	totalDuration := keyframes[len(keyframes)-1].Timestamp

	fso.logger.Debug("Optimizing segment boundaries",
		"target_duration", targetSegmentDuration,
		"total_duration", totalDuration,
		"keyframes", len(keyframes),
	)

	currentTime := time.Duration(0)
	for currentTime < totalDuration {
		targetTime := currentTime + targetSegmentDuration

		// Find keyframe closest to target boundary
		closestKeyframe := fso.findClosestKeyframe(keyframes, targetTime)
		if closestKeyframe == nil {
			break
		}

		// Only adjust if the keyframe is reasonably close (within 50% of segment duration)
		maxDeviation := time.Duration(targetDurationSeconds * 0.5 * float64(time.Second))
		if abs64(int64(targetTime)-int64(closestKeyframe.Timestamp)) <= int64(maxDeviation) {
			optimizedBoundaries = append(optimizedBoundaries, closestKeyframe.Timestamp)
			currentTime = closestKeyframe.Timestamp
		} else {
			// Use original boundary if no suitable keyframe
			optimizedBoundaries = append(optimizedBoundaries, targetTime)
			currentTime = targetTime
		}
	}

	fso.logger.Debug("Optimized boundaries",
		"original_segments", int(totalDuration.Seconds()/targetDurationSeconds),
		"optimized_segments", len(optimizedBoundaries),
	)

	return optimizedBoundaries
}

// AnalyzeSceneComplexity analyzes video complexity for adaptive segment sizing
func (fso *FastStartOptimizer) AnalyzeSceneComplexity(ctx context.Context, inputPath string) ([]float64, error) {
	fso.logger.Debug("Analyzing scene complexity", "input", inputPath)

	// Use ffprobe to get frame statistics
	args := []string{
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "frame=pkt_size,pict_type",
		"-of", "csv=print_section=0",
		inputPath,
	}

	cmd := exec.CommandContext(ctx, fso.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to analyze scene complexity: %w", err)
	}

	return fso.parseComplexityOutput(string(output))
}

// parseComplexityOutput calculates scene complexity from frame statistics
func (fso *FastStartOptimizer) parseComplexityOutput(output string) ([]float64, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var complexityScores []float64
	var frameSizes []int64

	// Collect frame sizes
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}

		size, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}

		frameSizes = append(frameSizes, size)
	}

	if len(frameSizes) == 0 {
		return complexityScores, nil
	}

	// Calculate complexity using sliding window variance
	windowSize := 30 // Analyze 30-frame windows (1 second at 30fps)

	for i := 0; i <= len(frameSizes)-windowSize; i++ {
		window := frameSizes[i : i+windowSize]
		complexity := fso.calculateVarianceScore(window)
		complexityScores = append(complexityScores, complexity)
	}

	fso.logger.Debug("Calculated complexity scores",
		"frames", len(frameSizes),
		"scores", len(complexityScores),
		"avg_complexity", fso.calculateAverage(complexityScores),
	)

	return complexityScores, nil
}

// calculateVarianceScore calculates complexity based on frame size variance
func (fso *FastStartOptimizer) calculateVarianceScore(frameSizes []int64) float64 {
	if len(frameSizes) == 0 {
		return 0.0
	}

	// Calculate mean
	var sum int64
	for _, size := range frameSizes {
		sum += size
	}
	mean := float64(sum) / float64(len(frameSizes))

	// Calculate variance
	var variance float64
	for _, size := range frameSizes {
		diff := float64(size) - mean
		variance += diff * diff
	}
	variance /= float64(len(frameSizes))

	// Normalize to 0-1 range (higher variance = higher complexity)
	// Using square root to reduce extreme values
	return variance / (mean * mean)
}

// GetFastStartProfile returns an optimized profile for fast startup
func (fso *FastStartOptimizer) GetFastStartProfile() OptimizationProfile {
	return OptimizationProfile{
		GOPSize:          30,    // 1-second GOP at 30fps
		KeyframeInterval: 30,    // Force keyframe every second
		SceneDetection:   false, // Disable for consistent timing

		MovAtom:      true, // Move moov atom to beginning
		FragmentHint: true, // Enable fragmentation
		FastStart:    true, // Enable fast-start

		SeekPreroll:      3, // 3 frames preroll
		IndexGranularity: 1, // Index every keyframe

		Profile: "main",       // Good compatibility
		Preset:  "fast",       // Fast encoding
		Tune:    "fastdecode", // Optimize for decode speed
	}
}

// GetSeekOptimizedProfile returns a profile optimized for seeking
func (fso *FastStartOptimizer) GetSeekOptimizedProfile() OptimizationProfile {
	return OptimizationProfile{
		GOPSize:          60,   // 2-second GOP
		KeyframeInterval: 60,   // Regular keyframes
		SceneDetection:   true, // Adaptive keyframes for scenes

		MovAtom:      true, // Fast start
		FragmentHint: true, // Better seeking
		FastStart:    true,

		SeekPreroll:      5, // More preroll for smooth seeks
		IndexGranularity: 2, // Index every 2 keyframes

		Profile: "high",       // Better compression
		Preset:  "medium",     // Balanced speed/quality
		Tune:    "fastdecode", // Decode optimization
	}
}

// ValidateOptimization checks if the optimization settings are working
func (fso *FastStartOptimizer) ValidateOptimization(ctx context.Context, outputPath string) (*OptimizationReport, error) {
	fso.logger.Debug("Validating optimization", "output", outputPath)

	// Analyze the output file
	keyframes, err := fso.AnalyzeKeyframes(ctx, outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate keyframes: %w", err)
	}

	// Check file structure
	hasFastStart, err := fso.checkFastStartOptimization(ctx, outputPath)
	if err != nil {
		fso.logger.Warn("Failed to check fast-start optimization", "error", err)
	}

	report := &OptimizationReport{
		FilePath:       outputPath,
		KeyframeCount:  len(keyframes),
		HasFastStart:   hasFastStart,
		SeekPoints:     fso.GenerateSeekPoints(keyframes, 4*time.Second),
		AverageGOP:     fso.calculateAverageGOP(keyframes),
		OptimizedSize:  fso.getFileSize(outputPath),
		ValidationTime: time.Now(),
	}

	// Calculate optimization score
	report.OptimizationScore = fso.calculateOptimizationScore(report)

	fso.logger.Debug("Optimization validation completed",
		"keyframes", report.KeyframeCount,
		"fast_start", report.HasFastStart,
		"seek_points", len(report.SeekPoints),
		"score", report.OptimizationScore,
	)

	return report, nil
}

// OptimizationReport contains validation results
type OptimizationReport struct {
	FilePath          string      `json:"file_path"`
	KeyframeCount     int         `json:"keyframe_count"`
	HasFastStart      bool        `json:"has_fast_start"`
	SeekPoints        []SeekPoint `json:"seek_points"`
	AverageGOP        float64     `json:"average_gop"`
	OptimizedSize     int64       `json:"optimized_size"`
	OptimizationScore float64     `json:"optimization_score"`
	ValidationTime    time.Time   `json:"validation_time"`
}

// Helper functions

func (fso *FastStartOptimizer) checkFastStartOptimization(ctx context.Context, filePath string) (bool, error) {
	// Use ffprobe to check if moov atom is at the beginning
	args := []string{
		"-v", "quiet",
		"-show_format",
		"-of", "csv=print_section=0",
		filePath,
	}

	cmd := exec.CommandContext(ctx, fso.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// Check for fast-start indicators in format info
	return strings.Contains(string(output), "faststart") ||
		strings.Contains(string(output), "frag_keyframe"), nil
}

func (fso *FastStartOptimizer) calculateAverageGOP(keyframes []KeyframeInfo) float64 {
	if len(keyframes) < 2 {
		return 0.0
	}

	var totalGOP float64
	for i := 1; i < len(keyframes); i++ {
		gop := keyframes[i].Timestamp.Seconds() - keyframes[i-1].Timestamp.Seconds()
		totalGOP += gop
	}

	return totalGOP / float64(len(keyframes)-1)
}

func (fso *FastStartOptimizer) calculateOptimizationScore(report *OptimizationReport) float64 {
	score := 0.0

	// Fast-start optimization (30% weight)
	if report.HasFastStart {
		score += 0.3
	}

	// Keyframe regularity (30% weight)
	targetGOP := 2.0 // 2-second GOP target
	gopDiff := abs64(int64(report.AverageGOP*1000) - int64(targetGOP*1000))
	gopScore := 1.0 - (float64(gopDiff) / 1000.0)
	if gopScore < 0 {
		gopScore = 0
	}
	score += 0.3 * gopScore

	// Seek point density (25% weight)
	if report.KeyframeCount > 0 {
		seekDensity := float64(len(report.SeekPoints)) / float64(report.KeyframeCount)
		if seekDensity > 1.0 {
			seekDensity = 1.0
		}
		score += 0.25 * seekDensity
	}

	// File size efficiency (15% weight)
	// This is a placeholder - would need baseline for comparison
	score += 0.15

	return score
}

func (fso *FastStartOptimizer) calculateAverageAccuracy(seekPoints []SeekPoint) float64 {
	if len(seekPoints) == 0 {
		return 0.0
	}

	var total float64
	for _, point := range seekPoints {
		total += point.Accuracy
	}

	return total / float64(len(seekPoints))
}

func (fso *FastStartOptimizer) calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func (fso *FastStartOptimizer) getFileSize(filePath string) int64 {
	if stat, err := os.Stat(filePath); err == nil {
		return stat.Size()
	}
	return 0
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
