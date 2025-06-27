package playbackmodule

import (
	"fmt"
	"sort"

	plugins "github.com/mantonx/viewra/sdk"
)

// BitrateProfile represents a single encoding profile in the ladder
type BitrateProfile struct {
	Name       string
	Width      int
	Height     int
	Bitrate    int    // Target bitrate in kbps
	MaxBitrate int    // Maximum bitrate in kbps
	BufferSize int    // VBV buffer size in kbps
	Profile    string // H.264 profile (baseline, main, high)
	Level      string // H.264 level
	CRF        int    // Constant Rate Factor for quality
}

// BitrateRung represents a single rung in the adaptive bitrate ladder
type BitrateRung struct {
	Resolution   plugins.Resolution
	VideoBitrate int    // kbps
	AudioBitrate int    // kbps
	Profile      string // codec profile
	Level        string // codec level
	CRF          int    // quality factor
	Label        string // e.g., "360p", "720p", "1080p"
}

// BitrateAdaptationSet represents a complete ABR ladder
type BitrateAdaptationSet struct {
	Rungs []BitrateRung
}

// GetOptimizedBitrateLadder returns an optimized bitrate ladder based on content and target quality
func GetOptimizedBitrateLadder(sourceWidth, sourceHeight int, maxQuality int) *BitrateAdaptationSet {
	ladder := &BitrateAdaptationSet{}

	// Calculate source aspect ratio
	aspectRatio := float64(sourceWidth) / float64(sourceHeight)

	// Define standard resolutions with optimized bitrates
	// These are based on industry best practices and VMAF optimization
	standardRungs := []struct {
		height       int
		minBitrate   int
		maxBitrate   int
		audioBitrate int
		profile      string
		level        string
		crf          int
		label        string
	}{
		// Ultra-low bitrate for poor connections
		{240, 200, 400, 64, "baseline", "3.0", 28, "240p"},
		// Low bitrate for mobile
		{360, 400, 800, 96, "baseline", "3.0", 26, "360p"},
		// Standard definition
		{480, 800, 1500, 128, "main", "3.1", 24, "480p"},
		// HD ready
		{720, 1500, 3000, 192, "high", "4.0", 23, "720p"},
		// Full HD
		{1080, 3000, 6000, 256, "high", "4.1", 22, "1080p"},
		// 2K
		{1440, 6000, 12000, 256, "high", "5.0", 21, "1440p"},
		// 4K
		{2160, 12000, 25000, 320, "high", "5.1", 20, "2160p"},
	}

	// Only include rungs up to source resolution
	for _, rung := range standardRungs {
		if rung.height > sourceHeight {
			break
		}

		// Calculate width maintaining aspect ratio
		width := int(float64(rung.height) * aspectRatio)
		// Round to even number for codec compatibility
		if width%2 != 0 {
			width++
		}

		// Adjust bitrate based on quality setting
		bitrate := rung.minBitrate + (rung.maxBitrate-rung.minBitrate)*maxQuality/100

		ladder.Rungs = append(ladder.Rungs, BitrateRung{
			Resolution: plugins.Resolution{
				Width:  width,
				Height: rung.height,
			},
			VideoBitrate: bitrate,
			AudioBitrate: rung.audioBitrate,
			Profile:      rung.profile,
			Level:        rung.level,
			CRF:          rung.crf,
			Label:        rung.label,
		})
	}

	// Always include at least the lowest quality rung for robustness
	if len(ladder.Rungs) == 0 {
		width := int(float64(240) * aspectRatio)
		if width%2 != 0 {
			width++
		}
		ladder.Rungs = append(ladder.Rungs, BitrateRung{
			Resolution: plugins.Resolution{
				Width:  width,
				Height: 240,
			},
			VideoBitrate: 300,
			AudioBitrate: 64,
			Profile:      "baseline",
			Level:        "3.0",
			CRF:          28,
			Label:        "240p",
		})
	}

	return ladder
}

// GetPerTitleOptimizedLadder returns a content-aware optimized ladder
// This is a simplified version - full per-title encoding would require content analysis
func GetPerTitleOptimizedLadder(contentType string, complexity float64, sourceWidth, sourceHeight int) *BitrateAdaptationSet {
	baseLadder := GetOptimizedBitrateLadder(sourceWidth, sourceHeight, 80)

	// Adjust bitrates based on content type and complexity
	complexityMultiplier := 1.0
	switch contentType {
	case "animation":
		// Animation typically compresses better
		complexityMultiplier = 0.7
	case "sports":
		// Sports need higher bitrates due to motion
		complexityMultiplier = 1.3
	case "film":
		// Film grain needs more bitrate
		complexityMultiplier = 1.1
	default:
		// Use complexity score if available
		if complexity > 0 {
			complexityMultiplier = 0.8 + (complexity * 0.4) // Range from 0.8 to 1.2
		}
	}

	// Apply complexity multiplier to bitrates
	for i := range baseLadder.Rungs {
		baseLadder.Rungs[i].VideoBitrate = int(float64(baseLadder.Rungs[i].VideoBitrate) * complexityMultiplier)
	}

	return baseLadder
}

// GenerateFFmpegCommandsForLadder generates FFmpeg commands for each rung in the ladder
func GenerateFFmpegCommandsForLadder(ladder *BitrateAdaptationSet, baseArgs []string) [][]string {
	var commands [][]string

	for i, rung := range ladder.Rungs {
		args := make([]string, len(baseArgs))
		copy(args, baseArgs)

		// Add resolution-specific arguments
		args = append(args,
			"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos", rung.Resolution.Width, rung.Resolution.Height),
			"-b:v", fmt.Sprintf("%dk", rung.VideoBitrate),
			"-maxrate", fmt.Sprintf("%dk", int(float64(rung.VideoBitrate)*1.5)), // 1.5x for VBV maxrate
			"-bufsize", fmt.Sprintf("%dk", rung.VideoBitrate*2), // 2x bitrate for buffer
			"-profile:v", rung.Profile,
			"-level", rung.Level,
			"-crf", fmt.Sprintf("%d", rung.CRF),
			"-b:a", fmt.Sprintf("%dk", rung.AudioBitrate),
		)

		// Add stream mapping for adaptive streaming
		args = append(args,
			"-adaptation_sets", fmt.Sprintf("id=%d,streams=v id=%d,streams=a", i, i+len(ladder.Rungs)),
		)

		commands = append(commands, args)
	}

	return commands
}

// SortLadderByBitrate sorts the ladder rungs by bitrate (ascending)
func SortLadderByBitrate(ladder *BitrateAdaptationSet) {
	sort.Slice(ladder.Rungs, func(i, j int) bool {
		return ladder.Rungs[i].VideoBitrate < ladder.Rungs[j].VideoBitrate
	})
}

// FilterLadderByBandwidth returns only rungs that fit within bandwidth constraints
func FilterLadderByBandwidth(ladder *BitrateAdaptationSet, maxBandwidth int) *BitrateAdaptationSet {
	filtered := &BitrateAdaptationSet{}

	for _, rung := range ladder.Rungs {
		totalBitrate := rung.VideoBitrate + rung.AudioBitrate
		if totalBitrate <= maxBandwidth {
			filtered.Rungs = append(filtered.Rungs, rung)
		}
	}

	// Always keep at least the lowest bitrate rung
	if len(filtered.Rungs) == 0 && len(ladder.Rungs) > 0 {
		filtered.Rungs = append(filtered.Rungs, ladder.Rungs[0])
	}

	return filtered
}

// GetLadderManifestInfo returns information needed for manifest generation
func GetLadderManifestInfo(ladder *BitrateAdaptationSet) map[string]interface{} {
	info := map[string]interface{}{
		"adaptationSets": []map[string]interface{}{},
	}

	videoSet := map[string]interface{}{
		"mimeType":        "video/mp4",
		"contentType":     "video",
		"representations": []map[string]interface{}{},
	}

	audioSet := map[string]interface{}{
		"mimeType":        "audio/mp4",
		"contentType":     "audio",
		"representations": []map[string]interface{}{},
	}

	for i, rung := range ladder.Rungs {
		// Video representation
		videoRep := map[string]interface{}{
			"id":        fmt.Sprintf("video_%d", i),
			"bandwidth": rung.VideoBitrate * 1000, // Convert to bps
			"width":     rung.Resolution.Width,
			"height":    rung.Resolution.Height,
			"codecs":    fmt.Sprintf("avc1.%s", getAVCCodecString(rung.Profile, rung.Level)),
			"label":     rung.Label,
		}
		videoSet["representations"] = append(videoSet["representations"].([]map[string]interface{}), videoRep)

		// Audio representation
		audioRep := map[string]interface{}{
			"id":                fmt.Sprintf("audio_%d", i),
			"bandwidth":         rung.AudioBitrate * 1000, // Convert to bps
			"codecs":            "mp4a.40.2",              // AAC-LC
			"audioSamplingRate": 48000,
		}
		audioSet["representations"] = append(audioSet["representations"].([]map[string]interface{}), audioRep)
	}

	info["adaptationSets"] = append(info["adaptationSets"].([]map[string]interface{}), videoSet, audioSet)

	return info
}

// getAVCCodecString returns the AVC codec string for DASH/HLS manifests
func getAVCCodecString(profile, level string) string {
	// Convert profile and level to hex values for codec string
	profileMap := map[string]string{
		"baseline": "42",
		"main":     "4D",
		"high":     "64",
	}

	levelMap := map[string]string{
		"3.0": "1E",
		"3.1": "1F",
		"4.0": "28",
		"4.1": "29",
		"5.0": "32",
		"5.1": "33",
	}

	profileHex := profileMap[profile]
	if profileHex == "" {
		profileHex = "42" // Default to baseline
	}

	levelHex := levelMap[level]
	if levelHex == "" {
		levelHex = "1E" // Default to 3.0
	}

	// Format: avc1.PPCCLL where PP=profile, CC=constraints, LL=level
	return fmt.Sprintf("%sE0%s", profileHex, levelHex)
}
