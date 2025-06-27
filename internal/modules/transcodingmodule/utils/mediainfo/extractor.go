// This utility provides media information extraction capabilities using FFprobe.
// It extracts essential metadata like duration, dimensions, codecs, and bitrates
// from video and audio files. This information is crucial for the transcoding
// pipeline to make informed decisions about encoding parameters and to properly
// report duration in streaming manifests.
//
// The extractor uses FFprobe's JSON output format for reliable parsing and
// handles various edge cases like missing fields or fractional frame rates.
package mediainfo

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MediaInfo represents extracted media information
type MediaInfo struct {
	Duration     float64 `json:"duration"`      // Duration in seconds
	Width        int     `json:"width"`         // Video width
	Height       int     `json:"height"`        // Video height
	VideoBitrate int     `json:"video_bitrate"` // Video bitrate in kbps
	AudioBitrate int     `json:"audio_bitrate"` // Audio bitrate in kbps
	VideoCodec   string  `json:"video_codec"`   // Video codec name
	AudioCodec   string  `json:"audio_codec"`   // Audio codec name
	FrameRate    float64 `json:"frame_rate"`    // Frame rate
	HasVideo     bool    `json:"has_video"`     // Whether file has video stream
	HasAudio     bool    `json:"has_audio"`     // Whether file has audio stream
	Container    string  `json:"container"`     // Container format
}

// Extractor extracts media information using FFprobe
type Extractor struct {
	ffprobePath string
}

// NewExtractor creates a new media info extractor
func NewExtractor() *Extractor {
	ffprobePath, _ := exec.LookPath("ffprobe")
	if ffprobePath == "" {
		ffprobePath = "ffprobe" // Fallback to PATH
	}
	return &Extractor{
		ffprobePath: ffprobePath,
	}
}

// ExtractInfo extracts media information from a file
func (e *Extractor) ExtractInfo(filePath string) (*MediaInfo, error) {
	// Use ffprobe to get media info in JSON format
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	}

	cmd := exec.Command(e.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var result struct {
		Format struct {
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
			FormatName string `json:"format_name"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			BitRate      string `json:"bit_rate"`
			AvgFrameRate string `json:"avg_frame_rate"`
			Duration     string `json:"duration"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &MediaInfo{
		Container: result.Format.FormatName,
	}

	// Parse format duration
	if result.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			info.Duration = dur
		}
	}

	// Process streams
	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			info.HasVideo = true
			info.VideoCodec = stream.CodecName
			info.Width = stream.Width
			info.Height = stream.Height

			// Parse video bitrate
			if stream.BitRate != "" {
				if br, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
					info.VideoBitrate = int(br / 1000) // Convert to kbps
				}
			}

			// Parse frame rate
			if stream.AvgFrameRate != "" {
				parts := strings.Split(stream.AvgFrameRate, "/")
				if len(parts) == 2 {
					if num, err1 := strconv.ParseFloat(parts[0], 64); err1 == nil {
						if den, err2 := strconv.ParseFloat(parts[1], 64); err2 == nil && den > 0 {
							info.FrameRate = num / den
						}
					}
				}
			}

			// Use stream duration if format duration not available
			if info.Duration == 0 && stream.Duration != "" {
				if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
					info.Duration = dur
				}
			}

		case "audio":
			info.HasAudio = true
			info.AudioCodec = stream.CodecName

			// Parse audio bitrate
			if stream.BitRate != "" {
				if br, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
					info.AudioBitrate = int(br / 1000) // Convert to kbps
				}
			}
		}
	}

	return info, nil
}

// GetDuration is a convenience method to just extract duration
func (e *Extractor) GetDuration(filePath string) (float64, error) {
	info, err := e.ExtractInfo(filePath)
	if err != nil {
		return 0, err
	}
	return info.Duration, nil
}
