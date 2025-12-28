// Package probe provides video metadata extraction using ffprobe.
//
// This package handles:
//   - Video metadata extraction (resolution, codec, duration, etc.)
//   - Audio/subtitle track extraction with detailed info
//   - HDR and color space detection
//
// The main entry point is [Client], which wraps ffprobe operations.
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// Client provides methods for extracting metadata using ffprobe.
type Client struct {
	paths *paths.Paths
}

// NewClient creates a new probe client.
// If paths is nil, creates new Paths from environment/PATH.
func NewClient(p *paths.Paths) (*Client, error) {
	var err error
	if p == nil {
		p, err = paths.New()
		if err != nil {
			return nil, err
		}
	}
	return &Client{paths: p}, nil
}

// VideoMetadata contains extracted metadata from a video file.
type VideoMetadata struct {
	Duration       time.Duration
	Width          int
	Height         int
	VideoCodec     string
	AudioCodec     string
	Bitrate        int64
	FrameRate      float64
	FileSize       int64
	CodecProfile   string
	ScanType       string
	HDRFormat      string
	ColorSpace     string
	ColorPrimaries string
}

// AudioTrackInfo contains metadata for an audio stream.
type AudioTrackInfo struct {
	StreamIndex   int
	Codec         string
	CodecProfile  string
	Channels      int
	ChannelLayout string
	SampleRate    int
	BitRate       int
	Language      string
	Title         string
	IsDefault     bool
	IsCommentary  bool
	IsDescriptive bool
}

// SubtitleTrackInfo contains metadata for a subtitle stream.
type SubtitleTrackInfo struct {
	StreamIndex  int
	Codec        string
	Language     string
	Title        string
	IsDefault    bool
	IsForced     bool
	IsSDH        bool
	IsCommentary bool
	IsBitmap     bool
}

// MediaTracksInfo contains all audio and subtitle tracks from a media file.
type MediaTracksInfo struct {
	AudioTracks    []AudioTrackInfo
	SubtitleTracks []SubtitleTrackInfo
}

// ffprobeOutput represents the JSON structure returned by ffprobe.
type ffprobeOutput struct {
	Format struct {
		Duration   string            `json:"duration"`
		Size       string            `json:"size"`
		BitRate    string            `json:"bit_rate"`
		FormatName string            `json:"format_name"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

// ffprobeStream represents a single stream in ffprobe output.
type ffprobeStream struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	RFrameRate     string            `json:"r_frame_rate"`
	BitRate        string            `json:"bit_rate"`
	FieldOrder     string            `json:"field_order"`
	ColorSpace     string            `json:"color_space"`
	ColorPrimaries string            `json:"color_primaries"`
	ColorTransfer  string            `json:"color_transfer"`
	SideDataList   []sideData        `json:"side_data_list,omitempty"`
	Tags           map[string]string `json:"tags"`
	// Audio-specific fields
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	SampleRate    string `json:"sample_rate"`
	// Stream disposition flags
	Disposition ffprobeDisposition `json:"disposition"`
}

// ffprobeDisposition represents stream disposition flags.
type ffprobeDisposition struct {
	Default         int `json:"default"`
	Forced          int `json:"forced"`
	Comment         int `json:"comment"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
}

// sideData represents side data in video streams (used for HDR metadata).
type sideData struct {
	SideDataType string `json:"side_data_type"`
}

// videoStream represents a video stream from ffprobe output.
type videoStream struct {
	CodecType      string
	CodecName      string
	Profile        string
	Width          int
	Height         int
	RFrameRate     string
	BitRate        string
	FieldOrder     string
	ColorSpace     string
	ColorPrimaries string
	ColorTransfer  string
	SideDataList   []sideData
	Tags           map[string]string
}

// ExtractMetadata extracts metadata from a video file using ffprobe.
func (c *Client) ExtractMetadata(ctx context.Context, filePath string) (*VideoMetadata, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrInvalidFile
	}

	cmd := c.paths.PrepareCommand("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create stdout pipe: %v", ErrMetadataExtraction, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	var probe ffprobeOutput
	decoder := json.NewDecoder(stdout)
	if err := decoder.Decode(&probe); err != nil {
		cmd.Wait()
		return nil, fmt.Errorf("%w: failed to parse ffprobe output: %v", ErrMetadataExtraction, err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	metadata := &VideoMetadata{}

	// Extract duration
	if probe.Format.Duration != "" {
		durationSec, err := strconv.ParseFloat(probe.Format.Duration, 64)
		if err == nil {
			metadata.Duration = time.Duration(durationSec * float64(time.Second))
		}
	}

	// Extract file size
	if probe.Format.Size != "" {
		fileSize, err := strconv.ParseInt(probe.Format.Size, 10, 64)
		if err == nil {
			metadata.FileSize = fileSize
		}
	}

	// Extract bitrate
	if probe.Format.BitRate != "" {
		bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64)
		if err == nil {
			metadata.Bitrate = bitrate
		}
	}

	// Track best video stream (highest resolution, excluding embedded thumbnails)
	var bestVideoStream *videoStream
	bestVideoResolution := 0

	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			// Skip MJPEG/PNG/BMP/GIF - typically embedded thumbnails/cover art
			codecLower := strings.ToLower(stream.CodecName)
			if codecLower == "mjpeg" || codecLower == "png" || codecLower == "bmp" || codecLower == "gif" {
				continue
			}

			resolution := stream.Width * stream.Height
			if resolution > bestVideoResolution {
				bestVideoResolution = resolution
				bestVideoStream = &videoStream{
					CodecType:      stream.CodecType,
					CodecName:      stream.CodecName,
					Profile:        stream.Profile,
					Width:          stream.Width,
					Height:         stream.Height,
					RFrameRate:     stream.RFrameRate,
					BitRate:        stream.BitRate,
					FieldOrder:     stream.FieldOrder,
					ColorSpace:     stream.ColorSpace,
					ColorPrimaries: stream.ColorPrimaries,
					ColorTransfer:  stream.ColorTransfer,
					SideDataList:   stream.SideDataList,
					Tags:           stream.Tags,
				}
			}

		case "audio":
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = stream.CodecName
			}
		}
	}

	// Apply best video stream metadata
	if bestVideoStream != nil {
		metadata.VideoCodec = bestVideoStream.CodecName
		metadata.Width = bestVideoStream.Width
		metadata.Height = bestVideoStream.Height

		// Parse frame rate (format: "num/den")
		if bestVideoStream.RFrameRate != "" {
			parts := strings.Split(bestVideoStream.RFrameRate, "/")
			if len(parts) == 2 {
				num, err1 := strconv.ParseFloat(parts[0], 64)
				den, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil && den != 0 {
					metadata.FrameRate = num / den
				}
			}
		}

		if bestVideoStream.Profile != "" {
			metadata.CodecProfile = bestVideoStream.Profile
		}

		metadata.ScanType = determineScanType(bestVideoStream.FieldOrder)

		if bestVideoStream.ColorSpace != "" {
			metadata.ColorSpace = bestVideoStream.ColorSpace
		}
		if bestVideoStream.ColorPrimaries != "" {
			metadata.ColorPrimaries = bestVideoStream.ColorPrimaries
		}

		metadata.HDRFormat = detectHDRFormat(*bestVideoStream)
	}

	return metadata, nil
}

// ExtractTracks extracts audio and subtitle track information from a media file.
func (c *Client) ExtractTracks(ctx context.Context, filePath string) (*MediaTracksInfo, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrInvalidFile
	}

	cmd := c.paths.PrepareCommand("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create stdout pipe: %v", ErrMetadataExtraction, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	var probe ffprobeOutput
	decoder := json.NewDecoder(stdout)
	if err := decoder.Decode(&probe); err != nil {
		cmd.Wait()
		return nil, fmt.Errorf("%w: failed to parse ffprobe output: %v", ErrMetadataExtraction, err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	tracks := &MediaTracksInfo{
		AudioTracks:    make([]AudioTrackInfo, 0),
		SubtitleTracks: make([]SubtitleTrackInfo, 0),
	}

	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "audio":
			track := parseAudioStream(stream)
			tracks.AudioTracks = append(tracks.AudioTracks, track)
		case "subtitle":
			track := parseSubtitleStream(stream)
			tracks.SubtitleTracks = append(tracks.SubtitleTracks, track)
		}
	}

	return tracks, nil
}

// parseAudioStream extracts audio track info from an ffprobe stream.
func parseAudioStream(stream ffprobeStream) AudioTrackInfo {
	track := AudioTrackInfo{
		StreamIndex:   stream.Index,
		Codec:         stream.CodecName,
		CodecProfile:  stream.Profile,
		Channels:      stream.Channels,
		ChannelLayout: stream.ChannelLayout,
		IsDefault:     stream.Disposition.Default == 1,
		IsCommentary:  stream.Disposition.Comment == 1,
		IsDescriptive: stream.Disposition.VisualImpaired == 1,
	}

	if stream.SampleRate != "" {
		if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
			track.SampleRate = sr
		}
	}

	if stream.BitRate != "" {
		if br, err := strconv.Atoi(stream.BitRate); err == nil {
			track.BitRate = br
		}
	}

	if stream.Tags != nil {
		if lang, ok := stream.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"]; ok {
			track.Title = title
			titleLower := strings.ToLower(title)
			if strings.Contains(titleLower, "commentary") || strings.Contains(titleLower, "comment") {
				track.IsCommentary = true
			}
			if strings.Contains(titleLower, "description") ||
				strings.Contains(titleLower, "descriptive") ||
				strings.Contains(titleLower, "visually impaired") {
				track.IsDescriptive = true
			}
		}
	}

	return track
}

// parseSubtitleStream extracts subtitle track info from an ffprobe stream.
func parseSubtitleStream(stream ffprobeStream) SubtitleTrackInfo {
	track := SubtitleTrackInfo{
		StreamIndex:  stream.Index,
		Codec:        stream.CodecName,
		IsDefault:    stream.Disposition.Default == 1,
		IsForced:     stream.Disposition.Forced == 1,
		IsSDH:        stream.Disposition.HearingImpaired == 1,
		IsCommentary: stream.Disposition.Comment == 1,
	}

	codecLower := strings.ToLower(stream.CodecName)
	track.IsBitmap = codecLower == "hdmv_pgs_subtitle" ||
		codecLower == "dvd_subtitle" ||
		codecLower == "dvdsub" ||
		codecLower == "pgssub" ||
		codecLower == "xsub"

	if stream.Tags != nil {
		if lang, ok := stream.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"]; ok {
			track.Title = title
			titleLower := strings.ToLower(title)
			if strings.Contains(titleLower, "sdh") ||
				strings.Contains(titleLower, "hearing impaired") ||
				strings.Contains(titleLower, "cc") {
				track.IsSDH = true
			}
			if strings.Contains(titleLower, "forced") ||
				strings.Contains(titleLower, "signs") ||
				strings.Contains(titleLower, "foreign") {
				track.IsForced = true
			}
			if strings.Contains(titleLower, "commentary") {
				track.IsCommentary = true
			}
		}
	}

	return track
}

// determineScanType determines if the video is progressive or interlaced.
func determineScanType(fieldOrder string) string {
	switch fieldOrder {
	case "progressive", "":
		return "progressive"
	case "tt", "bb", "tb", "bt":
		return "interlaced"
	default:
		return "progressive"
	}
}

// detectHDRFormat detects the HDR format from video stream metadata.
func detectHDRFormat(stream videoStream) string {
	// Check side_data_list first - most reliable source
	var hasHDR10Metadata bool
	for _, sideData := range stream.SideDataList {
		switch sideData.SideDataType {
		case "DOVI configuration record":
			// Dolby Vision detected via side_data - this is the most reliable method
			return "Dolby Vision"
		case "HDR10+ Application SEI":
			return "HDR10+"
		case "Mastering display metadata", "Content light level metadata":
			hasHDR10Metadata = true
		}
	}

	// Check profile for Dolby Vision indicators
	if strings.Contains(strings.ToLower(stream.Profile), "dolby") ||
		strings.Contains(strings.ToLower(stream.Profile), "dovi") {
		return "Dolby Vision"
	}

	// Check tags for Dolby Vision
	if tags := stream.Tags; tags != nil {
		if dolbyVision, ok := tags["DOVI_CONFIGURATION_RECORD"]; ok && dolbyVision != "" {
			return "Dolby Vision"
		}
	}

	// HDR10 detection - requires SMPTE 2084 (PQ) transfer function
	if hasHDR10Metadata && stream.ColorTransfer == "smpte2084" {
		return "HDR10"
	}

	if stream.ColorTransfer == "arib-std-b67" {
		return "HLG"
	}

	if stream.ColorTransfer == "smpte2084" {
		return "HDR10"
	}

	return ""
}
