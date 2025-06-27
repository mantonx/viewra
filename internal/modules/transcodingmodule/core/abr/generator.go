// Package abr provides adaptive bitrate ladder generation for video streaming.
// This generator creates optimized encoding profiles tailored to different devices,
// network conditions, and quality requirements. It ensures smooth playback across
// various platforms by intelligently selecting bitrates, resolutions, and codec
// settings that balance quality with bandwidth efficiency.
//
// The ABR ladder generation considers:
// - Source video characteristics (resolution, aspect ratio)
// - Target device capabilities (mobile, desktop, TV)
// - Network conditions (2G to fiber optic)
// - Quality preferences (bandwidth vs visual quality trade-offs)
//
// Example usage:
//
//	gen := abr.NewGenerator(logger)
//	ladder := gen.GenerateLadder(1920, 1080, 80) // 1080p source, 80% quality
//	for _, rung := range ladder {
//	    fmt.Printf("%s: %dx%d @ %dkbps\n", rung.Label, rung.Width, rung.Height, rung.VideoBitrate)
//	}
package abr

import (
	"github.com/hashicorp/go-hclog"
)

// BitrateLadderRung represents a single quality level in the ABR ladder
type BitrateLadderRung struct {
	Width        int
	Height       int
	VideoBitrate int    // kbps
	AudioBitrate int    // kbps
	Profile      string // H.264 profile
	Level        string // H.264 level
	CRF          int    // Constant Rate Factor
	Label        string // Human-readable label
	UseCase      string // Device/network type this targets
}

// Generator handles adaptive bitrate ladder generation
type Generator struct {
	logger hclog.Logger
}

// NewGenerator creates a new ABR ladder generator
func NewGenerator(logger hclog.Logger) *Generator {
	return &Generator{
		logger: logger,
	}
}

// GenerateLadder creates an optimized set of encoding profiles for different device types
func (g *Generator) GenerateLadder(sourceWidth, sourceHeight, quality int) []BitrateLadderRung {
	var ladder []BitrateLadderRung

	// Calculate aspect ratio
	aspectRatio := float64(sourceWidth) / float64(sourceHeight)

	// Define device-optimized ladder rungs
	// Order matters: lowest bandwidth first for fastest startup
	standardRungs := []struct {
		height       int
		videoBitrate int // kbps
		audioBitrate int // kbps
		profile      string
		level        string
		crf          int
		label        string
		useCase      string // Device/network type this targets
	}{
		// CPU-optimized ladder with fewer rungs
		// Comment: Reduced from 6 to 3 rungs to lower CPU usage by 50%
		// Mobile/Low bandwidth - covers 240p-480p use cases
		{480, 700, 96, "main", "3.1", 28, "480p", "mobile/WiFi"},
		// HD for standard viewing - most common use case
		{720, 1500, 96, "main", "4.0", 26, "720p", "broadband"},
		// Full HD for high quality - only when needed
		{1080, 2800, 128, "high", "4.1", 24, "1080p", "fiber/excellent"},
	}

	// Filter rungs based on source resolution and create adaptive ladder
	for _, rung := range standardRungs {
		// Don't exceed source resolution
		if rung.height > sourceHeight {
			continue
		}

		// Calculate proper width maintaining aspect ratio
		width := int(float64(rung.height) * aspectRatio)
		if width%2 != 0 {
			width++ // Ensure even width for video encoding
		}

		// Adjust bitrate based on quality setting (0-100 range)
		// Optimized for real-time streaming
		qualityMultiplier := float64(quality) / 80.0 // Normalize around 80% quality
		if qualityMultiplier < 0.5 {
			qualityMultiplier = 0.5 // Minimum quality threshold
		}
		if qualityMultiplier > 1.2 {
			qualityMultiplier = 1.2 // Maximum quality threshold for real-time
		}

		adjustedBitrate := int(float64(rung.videoBitrate) * qualityMultiplier)

		ladder = append(ladder, BitrateLadderRung{
			Width:        width,
			Height:       rung.height,
			VideoBitrate: adjustedBitrate,
			AudioBitrate: rung.audioBitrate,
			Profile:      rung.profile,
			Level:        rung.level,
			CRF:          rung.crf,
			Label:        rung.label,
			UseCase:      rung.useCase,
		})

		if g.logger != nil {
			g.logger.Debug("added ladder rung",
				"label", rung.label,
				"resolution", rung.height,
				"bitrate", adjustedBitrate,
				"use_case", rung.useCase,
			)
		}
	}

	// Always include at least the lowest rung
	if len(ladder) == 0 {
		width := int(float64(240) * aspectRatio)
		if width%2 != 0 {
			width++
		}
		ladder = append(ladder, BitrateLadderRung{
			Width:        width,
			Height:       240,
			VideoBitrate: 300,
			AudioBitrate: 64,
			Profile:      "baseline",
			Level:        "3.0",
			CRF:          28,
			Label:        "240p",
			UseCase:      "fallback",
		})

		if g.logger != nil {
			g.logger.Warn("source resolution too low, using minimum ladder rung",
				"source_height", sourceHeight,
			)
		}
	}

	if g.logger != nil {
		g.logger.Info("generated ABR ladder",
			"rungs", len(ladder),
			"source_resolution", sourceHeight,
			"quality_setting", quality,
		)
	}

	return ladder
}

// GetOptimalRung returns the best quality rung for a given bandwidth
func (g *Generator) GetOptimalRung(ladder []BitrateLadderRung, availableBandwidth int) *BitrateLadderRung {
	// Safety margin - use 80% of available bandwidth
	targetBandwidth := int(float64(availableBandwidth) * 0.8)

	var bestRung *BitrateLadderRung
	for i := range ladder {
		rung := &ladder[i]
		totalBitrate := rung.VideoBitrate + rung.AudioBitrate

		if totalBitrate <= targetBandwidth {
			bestRung = rung
		} else {
			break // Ladder is ordered by bitrate, so we can stop
		}
	}

	// If no rung fits, use the lowest
	if bestRung == nil && len(ladder) > 0 {
		bestRung = &ladder[0]
	}

	return bestRung
}

// CalculateStorageRequirements estimates storage needs for a given ladder
func (g *Generator) CalculateStorageRequirements(ladder []BitrateLadderRung, durationSeconds float64) map[string]interface{} {
	totalSizeMB := 0.0
	perRungSizes := make(map[string]float64)

	for _, rung := range ladder {
		// Calculate size for this rung
		totalBitrate := rung.VideoBitrate + rung.AudioBitrate
		sizeMB := (float64(totalBitrate) * durationSeconds) / (8 * 1024) // Convert kbps to MB

		perRungSizes[rung.Label] = sizeMB
		totalSizeMB += sizeMB
	}

	return map[string]interface{}{
		"total_size_mb":    totalSizeMB,
		"total_size_gb":    totalSizeMB / 1024,
		"per_rung_sizes":   perRungSizes,
		"duration_seconds": durationSeconds,
		"num_rungs":        len(ladder),
	}
}
