package utils

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/internal/types"
)

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Format  FFProbeFormat   `json:"format"`
	Streams []FFProbeStream `json:"streams"`
}

// FFProbeFormat represents format information from ffprobe
type FFProbeFormat struct {
	Filename       string            `json:"filename"`
	NBStreams      int               `json:"nb_streams"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	Tags           map[string]string `json:"tags"`
}

// FFProbeStream represents stream information from ffprobe
type FFProbeStream struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	CodecLongName  string            `json:"codec_long_name"`
	Profile        string            `json:"profile,omitempty"`
	CodecTagString string            `json:"codec_tag_string"`
	Width          int               `json:"width,omitempty"`
	Height         int               `json:"height,omitempty"`
	PixFmt         string            `json:"pix_fmt,omitempty"`
	FrameRate      string            `json:"r_frame_rate,omitempty"`
	BitRate        string            `json:"bit_rate,omitempty"`
	Channels       int               `json:"channels,omitempty"`
	ChannelLayout  string            `json:"channel_layout,omitempty"`
	SampleRate     string            `json:"sample_rate,omitempty"`
	Duration       string            `json:"duration,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// ProbeMediaFile uses ffprobe to extract media information
func ProbeMediaFile(filePath string) (*types.MediaInfo, error) {
	// Run ffprobe with JSON output
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	
	// Parse JSON output
	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	
	// Convert to MediaInfo
	mediaInfo := &types.MediaInfo{
		Path:      filePath,
		Container: strings.Split(probeOutput.Format.FormatName, ",")[0],
	}
	
	// Parse duration
	if duration, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
		mediaInfo.Duration = duration
	}
	
	// Parse size
	if size, err := strconv.ParseInt(probeOutput.Format.Size, 10, 64); err == nil {
		mediaInfo.Size = size
	}
	
	// Process streams
	for _, stream := range probeOutput.Streams {
		switch stream.CodecType {
		case "video":
			videoStream := types.VideoStream{
				Index:     stream.Index,
				Codec:     stream.CodecName,
				CodecLong: stream.CodecLongName,
				Profile:   stream.Profile,
				PixFmt:    stream.PixFmt,
				Width:     stream.Width,
				Height:    stream.Height,
				FrameRate: stream.FrameRate,
			}
			
			// Parse bitrate
			if bitrate, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
				videoStream.Bitrate = bitrate
			}
			
			// Calculate aspect ratio
			if stream.Width > 0 && stream.Height > 0 {
				gcd := getGCD(stream.Width, stream.Height)
				videoStream.AspectRatio = fmt.Sprintf("%d:%d", stream.Width/gcd, stream.Height/gcd)
			}
			
			mediaInfo.VideoStreams = append(mediaInfo.VideoStreams, videoStream)
			
		case "audio":
			audioStream := types.AudioStream{
				Index:         stream.Index,
				Codec:         stream.CodecName,
				CodecLong:     stream.CodecLongName,
				Channels:      stream.Channels,
				ChannelLayout: stream.ChannelLayout,
			}
			
			// Parse sample rate
			if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
				audioStream.SampleRate = sampleRate
			}
			
			// Parse bitrate
			if bitrate, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
				audioStream.Bitrate = bitrate
			}
			
			// Language from tags
			if lang, ok := stream.Tags["language"]; ok {
				audioStream.Language = lang
			}
			
			// Title from tags
			if title, ok := stream.Tags["title"]; ok {
				audioStream.Title = title
			}
			
			mediaInfo.AudioStreams = append(mediaInfo.AudioStreams, audioStream)
			
		case "subtitle":
			subtitle := types.SubtitleStream{
				Index: stream.Index,
				Codec: stream.CodecName,
			}
			
			// Language from tags
			if lang, ok := stream.Tags["language"]; ok {
				subtitle.Language = lang
			}
			
			// Title from tags  
			if title, ok := stream.Tags["title"]; ok {
				subtitle.Title = title
			}
			
			mediaInfo.Subtitles = append(mediaInfo.Subtitles, subtitle)
		}
	}
	
	return mediaInfo, nil
}

// getGCD calculates the greatest common divisor
func getGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// GetMediaDuration returns the duration of a media file in seconds
func GetMediaDuration(filePath string) (float64, error) {
	info, err := ProbeMediaFile(filePath)
	if err != nil {
		return 0, err
	}
	return info.Duration, nil
}

// IsAudioFile checks if a file is an audio file based on its streams
func IsAudioFile(filePath string) (bool, error) {
	info, err := ProbeMediaFile(filePath)
	if err != nil {
		return false, err
	}
	
	// File is audio if it has audio streams but no video streams
	return len(info.AudioStreams) > 0 && len(info.VideoStreams) == 0, nil
}

// IsVideoFile checks if a file is a video file based on its streams
func IsVideoFile(filePath string) (bool, error) {
	info, err := ProbeMediaFile(filePath)
	if err != nil {
		return false, err
	}
	
	// File is video if it has video streams
	return len(info.VideoStreams) > 0, nil
}