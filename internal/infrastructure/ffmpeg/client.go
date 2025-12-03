package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// NewClient creates a new FFmpeg client.
// It checks for the presence of ffmpeg and ffprobe executables in the system PATH.
// NOTE: This should only be called once and the client reused to avoid repeated PATH searches.
// The Coordinator creates this once and reuses it for all files.
func NewClient() (*Client, error) {
	// Cache the paths so they're only looked up once per client instance
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, ErrFFmpegNotFound
	}

	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, ErrFFprobeNotFound
	}

	return &Client{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}, nil
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

// sideData represents side data in video streams (used for HDR metadata)
type sideData struct {
	SideDataType string `json:"side_data_type"`
}

// videoStream represents a video stream from ffprobe output
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
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrInvalidFile
	}

	// Run ffprobe with JSON output
	cmd := exec.CommandContext(ctx, c.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	// Stream parse JSON to avoid loading entire output into memory
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create stdout pipe: %v", ErrMetadataExtraction, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	// Parse JSON output incrementally from stream
	var probe ffprobeOutput
	decoder := json.NewDecoder(stdout)
	if err := decoder.Decode(&probe); err != nil {
		cmd.Wait() // Clean up process
		return nil, fmt.Errorf("%w: failed to parse ffprobe output: %v", ErrMetadataExtraction, err)
	}

	// Wait for command to complete
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

	// Extract video and audio stream information
	// Track best video stream (highest resolution, excluding embedded thumbnails)
	var bestVideoStream *videoStream
	bestVideoResolution := 0

	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			// Skip MJPEG streams - these are typically embedded thumbnails/cover art in MKV files
			// Also skip PNG and BMP which can be used for cover art
			codecLower := strings.ToLower(stream.CodecName)
			if codecLower == "mjpeg" || codecLower == "png" || codecLower == "bmp" || codecLower == "gif" {
				continue
			}

			// Calculate resolution (width * height)
			resolution := stream.Width * stream.Height

			// Keep track of the highest resolution video stream
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
			if metadata.AudioCodec == "" { // Take first audio stream
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

		// Extract codec profile
		if bestVideoStream.Profile != "" {
			metadata.CodecProfile = bestVideoStream.Profile
		}

		// Extract scan type (progressive/interlaced)
		metadata.ScanType = determineScanType(bestVideoStream.FieldOrder)

		// Extract color metadata
		if bestVideoStream.ColorSpace != "" {
			metadata.ColorSpace = bestVideoStream.ColorSpace
		}
		if bestVideoStream.ColorPrimaries != "" {
			metadata.ColorPrimaries = bestVideoStream.ColorPrimaries
		}

		// Detect HDR format
		metadata.HDRFormat = detectHDRFormat(*bestVideoStream)
	}

	return metadata, nil
}

// GenerateThumbnail generates a thumbnail image from a video file.
// The output path should include the desired image extension (e.g., .jpg, .png).
func (c *Client) GenerateThumbnail(ctx context.Context, videoPath, outputPath string, opts ThumbnailOptions) error {
	// Check if file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return ErrInvalidFile
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("%w: failed to create output directory: %v", ErrThumbnailGeneration, err)
	}

	// Set default quality if not specified
	quality := opts.Quality
	if quality == 0 {
		quality = 2
	}

	// Build ffmpeg command
	args := []string{
		"-ss", formatDuration(opts.Timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", strconv.Itoa(quality),
	}

	// Add scaling if width or height specified
	if opts.Width > 0 || opts.Height > 0 {
		width := opts.Width
		height := opts.Height
		if width == 0 {
			width = -1 // Auto-scale width
		}
		if height == 0 {
			height = -1 // Auto-scale height
		}
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	}

	args = append(args, "-y", outputPath)

	cmd := exec.CommandContext(ctx, c.ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %v: %s", ErrThumbnailGeneration, err, string(output))
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: output file was not created", ErrThumbnailGeneration)
	}

	return nil
}

// formatDuration formats a time.Duration into a string suitable for ffmpeg (HH:MM:SS.mmm).
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

// determineScanType determines if the video is progressive or interlaced based on field order.
func determineScanType(fieldOrder string) string {
	switch fieldOrder {
	case "progressive", "":
		return "progressive"
	case "tt", "bb", "tb", "bt":
		return "interlaced"
	default:
		return "progressive" // Default to progressive if unknown
	}
}

// detectHDRFormat detects the HDR format from video stream metadata.
func detectHDRFormat(stream videoStream) string {
	// Check for HDR10/HDR10+ via side data
	for _, sideData := range stream.SideDataList {
		switch sideData.SideDataType {
		case "HDR10+ Application SEI":
			return "HDR10+"
		case "Mastering display metadata", "Content light level metadata":
			// These indicate HDR10
			if stream.ColorTransfer == "smpte2084" { // PQ transfer function
				return "HDR10"
			}
		}
	}

	// Check for Dolby Vision via codec profile
	if strings.Contains(strings.ToLower(stream.Profile), "dolby") ||
		strings.Contains(strings.ToLower(stream.Profile), "dovi") {
		return "Dolby Vision"
	}

	// Check for HLG (Hybrid Log-Gamma)
	if stream.ColorTransfer == "arib-std-b67" {
		return "HLG"
	}

	// Check via color transfer function
	if stream.ColorTransfer == "smpte2084" {
		return "HDR10"
	}

	// Check tags for HDR indicators
	if tags := stream.Tags; tags != nil {
		if dolbyVision, ok := tags["DOVI_CONFIGURATION_RECORD"]; ok && dolbyVision != "" {
			return "Dolby Vision"
		}
	}

	return "" // No HDR detected
}

// ExtractTracks extracts audio and subtitle track information from a media file.
func (c *Client) ExtractTracks(ctx context.Context, filePath string) (*MediaTracksInfo, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrInvalidFile
	}

	cmd := exec.CommandContext(ctx, c.ffprobePath,
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

	// Parse sample rate
	if stream.SampleRate != "" {
		if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
			track.SampleRate = sr
		}
	}

	// Parse bitrate
	if stream.BitRate != "" {
		if br, err := strconv.Atoi(stream.BitRate); err == nil {
			track.BitRate = br
		}
	}

	// Extract language and title from tags
	if stream.Tags != nil {
		if lang, ok := stream.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"]; ok {
			track.Title = title
			// Detect commentary from title
			titleLower := strings.ToLower(title)
			if strings.Contains(titleLower, "commentary") ||
				strings.Contains(titleLower, "comment") {
				track.IsCommentary = true
			}
			// Detect audio description from title
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
		StreamIndex: stream.Index,
		Codec:       stream.CodecName,
		IsDefault:   stream.Disposition.Default == 1,
		IsForced:    stream.Disposition.Forced == 1,
		IsSDH:       stream.Disposition.HearingImpaired == 1,
		IsCommentary: stream.Disposition.Comment == 1,
	}

	// Detect bitmap-based subtitles
	codecLower := strings.ToLower(stream.CodecName)
	track.IsBitmap = codecLower == "hdmv_pgs_subtitle" ||
		codecLower == "dvd_subtitle" ||
		codecLower == "dvdsub" ||
		codecLower == "pgssub" ||
		codecLower == "xsub"

	// Extract language and title from tags
	if stream.Tags != nil {
		if lang, ok := stream.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"]; ok {
			track.Title = title
			titleLower := strings.ToLower(title)
			// Detect SDH from title
			if strings.Contains(titleLower, "sdh") ||
				strings.Contains(titleLower, "hearing impaired") ||
				strings.Contains(titleLower, "cc") {
				track.IsSDH = true
			}
			// Detect forced from title
			if strings.Contains(titleLower, "forced") ||
				strings.Contains(titleLower, "signs") ||
				strings.Contains(titleLower, "foreign") {
				track.IsForced = true
			}
			// Detect commentary from title
			if strings.Contains(titleLower, "commentary") {
				track.IsCommentary = true
			}
		}
	}

	return track
}
