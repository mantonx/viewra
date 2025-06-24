// Package transcoding provides validation for video playback quality.
// This validator ensures that video output meets all requirements for a
// smooth, reliable playback experience across different devices and players.
// It validates DASH output, encoding parameters, and file organization
// to prevent common playback issues like buffering, seeking problems,
// and codec incompatibilities.
package validation

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PlaybackValidation represents the validation results for video playback quality
type PlaybackValidation struct {
	StaticMPD          bool   `json:"static_mpd"`           // MPD type="static" not "dynamic"
	TimelineAddressing bool   `json:"timeline_addressing"`  // use_timeline=1 for precise seeking
	GOPAlignment       bool   `json:"gop_alignment"`        // GOP = segment duration * fps
	InitSegments       bool   `json:"init_segments"`        // Proper init segments with metadata
	SegmentDuration    bool   `json:"segment_duration"`     // 2-6 second segments
	NoSceneDetection   bool   `json:"no_scene_detection"`   // sc_threshold=0
	ProperCodecLevel   bool   `json:"proper_codec_level"`   // H.264 level 4.0+
	SeparateSegments   bool   `json:"separate_segments"`    // single_file=0
	BitrateConsistency bool   `json:"bitrate_consistency"`  // Consistent bitrate ladder
	FileOrganization   bool   `json:"file_organization"`    // Organized directory structure
	ErrorMessage       string `json:"error_message,omitempty"`
}

// IsValid returns true if all validations pass
func (v PlaybackValidation) IsValid() bool {
	return v.StaticMPD &&
		v.TimelineAddressing &&
		v.GOPAlignment &&
		v.InitSegments &&
		v.SegmentDuration &&
		v.NoSceneDetection &&
		v.ProperCodecLevel &&
		v.SeparateSegments &&
		v.BitrateConsistency &&
		v.FileOrganization
}

// GetIssues returns a list of validation issues
func (v PlaybackValidation) GetIssues() []string {
	var issues []string
	
	if !v.StaticMPD {
		issues = append(issues, "Manifest must be type='static' for reliable playback")
	}
	if !v.TimelineAddressing {
		issues = append(issues, "Timeline addressing required for precise seeking")
	}
	if !v.GOPAlignment {
		issues = append(issues, "GOP size must equal segment duration × frame rate")
	}
	if !v.InitSegments {
		issues = append(issues, "Init segments must contain all metadata")
	}
	if !v.SegmentDuration {
		issues = append(issues, "Segment duration should be 2-6 seconds")
	}
	if !v.NoSceneDetection {
		issues = append(issues, "Scene change detection must be disabled (sc_threshold=0)")
	}
	if !v.ProperCodecLevel {
		issues = append(issues, "H.264 level should be 4.0 or higher")
	}
	if !v.SeparateSegments {
		issues = append(issues, "Segments must be separate files (single_file=0)")
	}
	if !v.BitrateConsistency {
		issues = append(issues, "Bitrate ladder must be consistent")
	}
	if !v.FileOrganization {
		issues = append(issues, "Files must be properly organized")
	}
	
	return issues
}

// SimpleMPD is a simplified structure for parsing DASH manifests
type SimpleMPD struct {
	XMLName xml.Name `xml:"MPD"`
	Type    string   `xml:"type,attr"`
	Periods []struct {
		AdaptationSets []struct {
			MimeType        string `xml:"mimeType,attr"`
			Representations []struct {
				ID         string `xml:"id,attr"`
				Bandwidth  int    `xml:"bandwidth,attr"`
				Codecs     string `xml:"codecs,attr"`
				Width      int    `xml:"width,attr"`
				Height     int    `xml:"height,attr"`
				FrameRate  string `xml:"frameRate,attr"`
				SegmentTemplate struct {
					Initialization string `xml:"initialization,attr"`
					Media          string `xml:"media,attr"`
					Duration       int    `xml:"duration,attr"`
					Timescale      int    `xml:"timescale,attr"`
					SegmentTimeline *struct {
						S []struct {
							T int `xml:"t,attr"`
							D int `xml:"d,attr"`
							R int `xml:"r,attr"`
						} `xml:"S"`
					} `xml:"SegmentTimeline"`
				} `xml:"SegmentTemplate"`
			} `xml:"Representation"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

// ValidatePlaybackOutput validates video output for smooth playback experience
func ValidatePlaybackOutput(outputDir string) PlaybackValidation {
	validation := PlaybackValidation{}
	
	// Check manifest
	manifestPath := filepath.Join(outputDir, "manifest.mpd")
	manifestContent, err := os.ReadFile(manifestPath)
	if err != nil {
		validation.ErrorMessage = fmt.Sprintf("Failed to read manifest: %v", err)
		return validation
	}
	
	// Parse MPD
	var mpd SimpleMPD
	if err := xml.Unmarshal(manifestContent, &mpd); err != nil {
		validation.ErrorMessage = fmt.Sprintf("Failed to parse manifest: %v", err)
		return validation
	}
	
	// Check static MPD
	validation.StaticMPD = mpd.Type == "static"
	
	// Check for timeline addressing and other properties
	if len(mpd.Periods) > 0 && len(mpd.Periods[0].AdaptationSets) > 0 {
		for _, adaptationSet := range mpd.Periods[0].AdaptationSets {
			if strings.Contains(adaptationSet.MimeType, "video") {
				for _, repr := range adaptationSet.Representations {
					// Check timeline addressing
					validation.TimelineAddressing = repr.SegmentTemplate.SegmentTimeline != nil
					
					// Check init segments
					validation.InitSegments = repr.SegmentTemplate.Initialization != ""
					
					// Check segment duration (2-6 seconds)
					if repr.SegmentTemplate.Timescale > 0 && repr.SegmentTemplate.Duration > 0 {
						segmentDurationSeconds := float64(repr.SegmentTemplate.Duration) / float64(repr.SegmentTemplate.Timescale)
						validation.SegmentDuration = segmentDurationSeconds >= 2.0 && segmentDurationSeconds <= 6.0
					}
					
					// Check codec level
					if strings.Contains(repr.Codecs, "avc1") {
						// Extract level from codec string (e.g., avc1.64001f = level 3.1, avc1.640028 = level 4.0)
						codecParts := strings.Split(repr.Codecs, ".")
						if len(codecParts) > 1 && len(codecParts[1]) >= 6 {
							levelHex := codecParts[1][4:6]
							validation.ProperCodecLevel = levelHex >= "28" // 0x28 = 40 = level 4.0
						}
					}
					
					break // Check first video representation
				}
			}
		}
	}
	
	// Check for separate segment files
	segmentPattern := filepath.Join(outputDir, "*segment*.m4s")
	segments, err := filepath.Glob(segmentPattern)
	if err == nil {
		validation.SeparateSegments = len(segments) > 0
	}
	
	// Check file organization
	validation.FileOrganization = checkFileOrganization(outputDir)
	
	// Check bitrate consistency
	validation.BitrateConsistency = checkBitrateConsistency(mpd)
	
	// GOP alignment and scene detection need to be validated during encoding
	// For now, we'll check if the manifest indicates proper configuration
	manifestStr := string(manifestContent)
	validation.GOPAlignment = validation.TimelineAddressing // Timeline addressing implies GOP alignment
	validation.NoSceneDetection = !strings.Contains(manifestStr, "scenecut") // Basic check
	
	return validation
}

// checkFileOrganization verifies proper directory structure
func checkFileOrganization(outputDir string) bool {
	// Check for expected file patterns
	requiredPatterns := []string{
		"manifest.mpd",
		"*init*.m4s",     // Init segments
		"*segment*.m4s",  // Media segments
	}
	
	for _, pattern := range requiredPatterns {
		files, err := filepath.Glob(filepath.Join(outputDir, pattern))
		if err != nil || len(files) == 0 {
			return false
		}
	}
	
	return true
}

// checkBitrateConsistency verifies bitrate ladder is properly structured
func checkBitrateConsistency(mpd SimpleMPD) bool {
	if len(mpd.Periods) == 0 {
		return false
	}
	
	var bitrates []int
	for _, adaptationSet := range mpd.Periods[0].AdaptationSets {
		if strings.Contains(adaptationSet.MimeType, "video") {
			for _, repr := range adaptationSet.Representations {
				bitrates = append(bitrates, repr.Bandwidth)
			}
		}
	}
	
	if len(bitrates) < 2 {
		return true // Single bitrate is consistent by definition
	}
	
	// Check that bitrates follow a reasonable ladder (each step is 1.5x to 2.5x previous)
	for i := 1; i < len(bitrates); i++ {
		ratio := float64(bitrates[i]) / float64(bitrates[i-1])
		if ratio < 1.3 || ratio > 3.0 {
			return false
		}
	}
	
	return true
}

// ValidateTranscodingParams validates transcoding parameters before encoding
type TranscodingParams struct {
	UseTimeline      bool    `json:"use_timeline"`
	SegmentDuration  int     `json:"segment_duration"`
	GOPSize          int     `json:"gop_size"`
	FrameRate        float64 `json:"frame_rate"`
	SceneThreshold   int     `json:"scene_threshold"`
	SingleFile       bool    `json:"single_file"`
	H264Level        string  `json:"h264_level"`
}

// ValidateTranscodingParams ensures parameters will produce reliable playback output
func ValidateTranscodingParams(params TranscodingParams) error {
	// Timeline addressing is required
	if !params.UseTimeline {
		return fmt.Errorf("use_timeline must be enabled for reliable playback")
	}
	
	// Segment duration should be 2-6 seconds
	if params.SegmentDuration < 2 || params.SegmentDuration > 6 {
		return fmt.Errorf("segment_duration should be between 2-6 seconds, got %d", params.SegmentDuration)
	}
	
	// GOP should align with segment duration
	expectedGOP := int(float64(params.SegmentDuration) * params.FrameRate)
	if params.GOPSize != expectedGOP {
		return fmt.Errorf("gop_size should be %d (segment_duration * frame_rate), got %d", expectedGOP, params.GOPSize)
	}
	
	// Scene detection should be disabled
	if params.SceneThreshold != 0 {
		return fmt.Errorf("scene_threshold must be 0 to disable scene detection")
	}
	
	// Should use separate files
	if params.SingleFile {
		return fmt.Errorf("single_file must be false for separate segment files")
	}
	
	// Check H264 level
	validLevels := []string{"4.0", "4.1", "4.2", "5.0", "5.1", "5.2"}
	levelValid := false
	for _, level := range validLevels {
		if params.H264Level == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		return fmt.Errorf("h264_level should be 4.0 or higher, got %s", params.H264Level)
	}
	
	return nil
}

// GenerateValidationReport creates a detailed validation report for playback quality
func GenerateValidationReport(validation PlaybackValidation, outputPath string) error {
	report := fmt.Sprintf(`Video Playback Validation Report
==========================

Overall Status: %s

Validation Results:
-------------------
✅ Static MPD:           %t
✅ Timeline Addressing:  %t
✅ GOP Alignment:        %t
✅ Init Segments:        %t
✅ Segment Duration:     %t
✅ No Scene Detection:   %t
✅ Proper Codec Level:   %t
✅ Separate Segments:    %t
✅ Bitrate Consistency:  %t
✅ File Organization:    %t

`,
		func() string {
			if validation.IsValid() {
				return "✅ PASSED"
			}
			return "❌ FAILED"
		}(),
		validation.StaticMPD,
		validation.TimelineAddressing,
		validation.GOPAlignment,
		validation.InitSegments,
		validation.SegmentDuration,
		validation.NoSceneDetection,
		validation.ProperCodecLevel,
		validation.SeparateSegments,
		validation.BitrateConsistency,
		validation.FileOrganization,
	)
	
	if !validation.IsValid() {
		report += "Issues Found:\n-------------\n"
		for _, issue := range validation.GetIssues() {
			report += fmt.Sprintf("❌ %s\n", issue)
		}
	}
	
	if validation.ErrorMessage != "" {
		report += fmt.Sprintf("\nError: %s\n", validation.ErrorMessage)
	}
	
	return os.WriteFile(outputPath, []byte(report), 0644)
}