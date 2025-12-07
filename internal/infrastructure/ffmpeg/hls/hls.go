// Package hls provides FFmpeg command building for HLS transcoding.
//
// This package handles:
//   - FFmpeg argument building for HLS output
//   - Codec selection and hardware acceleration
//   - Video filters (scaling, tone mapping)
//   - Audio encoding configuration
//
// The main entry point is [Builder], a fluent interface for constructing
// FFmpeg command-line arguments.
//
// Example:
//
//	opts := hls.Options{
//	    InputPath:  "/path/to/video.mkv",
//	    OutputDir:  "/cache/transcodes/123/1080p",
//	    Profile:    profile,
//	    VideoCodec: hls.CodecH264,
//	}
//	builder := hls.NewBuilder(opts)
//	args := builder.
//	    AddInput().
//	    AddStreamMapping().
//	    AddVideoEncoding().
//	    AddAudioEncoding().
//	    AddHLSOutput().
//	    AddOutputFile().
//	    Build()
package hls

import (
	"path/filepath"
	"strconv"
	"strings"
)

// SegmentFilenameFormat is the format string for segment filenames.
const SegmentFilenameFormat = "seg_%06d.ts"

// Builder provides a fluent interface for building FFmpeg command arguments.
// This eliminates code duplication across the 4 different transcoding strategies.
type Builder struct {
	args []string
	opts Options
}

// NewBuilder creates a new FFmpeg arguments builder.
func NewBuilder(opts Options) *Builder {
	return &Builder{
		args: []string{},
		opts: opts,
	}
}

// Build returns the final arguments slice.
func (b *Builder) Build() []string {
	return b.args
}

// Add appends raw arguments to the builder.
// Use this sparingly - prefer the typed methods.
func (b *Builder) Add(args ...string) *Builder {
	b.args = append(b.args, args...)
	return b
}

// AddLogLevel sets the FFmpeg log level.
func (b *Builder) AddLogLevel(level string) *Builder {
	b.args = append(b.args, "-loglevel", level)
	return b
}

// AddHideBanner hides the FFmpeg banner.
func (b *Builder) AddHideBanner() *Builder {
	b.args = append(b.args, "-hide_banner")
	return b
}

// AddNostdin disables stdin interaction.
func (b *Builder) AddNostdin() *Builder {
	b.args = append(b.args, "-nostdin")
	return b
}

// AddTimestampHandling adds timestamp handling for seeking.
// Uses -copyts and -start_at_zero to preserve original timestamps while starting from zero.
func (b *Builder) AddTimestampHandling() *Builder {
	b.args = append(b.args, "-copyts", "-start_at_zero")
	return b
}

// AddSeekPosition adds seek position (-ss) before input.
// Position is in seconds.
func (b *Builder) AddSeekPosition(seconds int) *Builder {
	if seconds > 0 {
		b.args = append(b.args, "-ss", strconv.Itoa(seconds))
	}
	return b
}

// AddNoAccurateSeek disables accurate seek for faster startup.
func (b *Builder) AddNoAccurateSeek() *Builder {
	b.args = append(b.args, "-noaccurate_seek")
	return b
}

// AddHardwareAccel adds hardware acceleration arguments.
// These must be added before the input file.
func (b *Builder) AddHardwareAccel(args []string) *Builder {
	b.args = append(b.args, args...)
	return b
}

// AddInput adds the input file.
func (b *Builder) AddInput() *Builder {
	b.args = append(b.args, "-i", b.opts.InputPath)
	return b
}

// AddStreamMapping adds stream mapping for video and audio.
// If a specific audio track is configured, maps that track.
func (b *Builder) AddStreamMapping() *Builder {
	b.args = append(b.args, "-map", "0:v:0")

	if b.opts.UseSpecificAudioTrack {
		b.args = append(b.args, "-map", "0:a:"+strconv.Itoa(b.opts.AudioTrackIndex))
	} else {
		b.args = append(b.args, "-map", "0:a:0")
	}

	return b
}

// AddProgressReporting adds progress reporting to stderr.
// Output format: "frame=N fps=X.X q=X.X size=NkB time=HH:MM:SS.ss bitrate=NkB/s speed=Xx"
func (b *Builder) AddProgressReporting() *Builder {
	b.args = append(b.args, "-progress", "pipe:2")
	return b
}

// AddMemorySafetyOptions adds options to prevent excessive memory allocation.
// Uses -max_alloc to limit single allocation size (prevents OOM on corrupt files).
func (b *Builder) AddMemorySafetyOptions(maxAllocMB int) *Builder {
	if maxAllocMB > 0 {
		maxAllocBytes := maxAllocMB * 1024 * 1024
		b.args = append(b.args, "-max_alloc", strconv.Itoa(maxAllocBytes))
	}
	return b
}

// PlaylistMetadata contains custom metadata from an HLS playlist.
// These values are written by our patched FFmpeg (patch 0007) and indicate
// the actual keyframe position when seeking.
type PlaylistMetadata struct {
	// StartPTSMs is the actual start PTS in milliseconds from the source file.
	// When seeking to 3530s but landing on a keyframe at 3527.5s, this will be 3527500.
	StartPTSMs int64

	// StartOffsetSec is the offset from the requested seek time in seconds.
	// Negative means playback starts before the requested time (landed on earlier keyframe).
	// Positive means playback starts after (landed on later keyframe).
	StartOffsetSec float64

	// HasStartInfo indicates whether the playlist contains start PTS information.
	// This will be false for playlists generated without seeking or with stock FFmpeg.
	HasStartInfo bool
}

// ParsePlaylistMetadata reads custom extension tags from an M3U8 playlist.
// Returns metadata with HasStartInfo=false if the tags are not present.
func ParsePlaylistMetadata(playlistContent string) PlaylistMetadata {
	meta := PlaylistMetadata{}

	for _, line := range strings.Split(playlistContent, "\n") {
		line = strings.TrimSpace(line)

		// Parse #EXT-X-START-PTS:<milliseconds>
		if strings.HasPrefix(line, "#EXT-X-START-PTS:") {
			value := strings.TrimPrefix(line, "#EXT-X-START-PTS:")
			if pts, err := strconv.ParseInt(value, 10, 64); err == nil {
				meta.StartPTSMs = pts
				meta.HasStartInfo = true
			}
		}

		// Parse #EXT-X-START-OFFSET:<seconds>
		if strings.HasPrefix(line, "#EXT-X-START-OFFSET:") {
			value := strings.TrimPrefix(line, "#EXT-X-START-OFFSET:")
			if offset, err := strconv.ParseFloat(value, 64); err == nil {
				meta.StartOffsetSec = offset
			}
		}
	}

	return meta
}

// AddHLSOutput adds HLS output settings (without output file - use AddOutputFile() separately).
// Uses MPEG-TS segments for maximum compatibility and fast startup.
func (b *Builder) AddHLSOutput() *Builder {
	p := b.opts.Profile
	segmentPath := filepath.Join(b.opts.OutputDir, SegmentFilenameFormat)

	// Calculate start segment number based on seek position
	startSegmentNum := 0
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		startSegmentNum = b.opts.StartPosition / p.SegmentDuration
	}

	b.args = append(b.args,
		"-f", "hls",
		"-hls_init_time", "1",
		"-hls_time", strconv.Itoa(p.SegmentDuration),
		"-hls_playlist_type", "event",
		"-hls_segment_filename", segmentPath,
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments+split_by_time",
		"-start_number", strconv.Itoa(startSegmentNum),
	)

	return b
}

// AddSegmentMuxerOutput adds segment muxer output settings for HLS.
// This uses FFmpeg's segment muxer instead of the HLS muxer, which provides better
// control over segment timing when seeking.
func (b *Builder) AddSegmentMuxerOutput() *Builder {
	p := b.opts.Profile
	playlistPath := filepath.Join(b.opts.OutputDir, "playlist.m3u8")

	// Calculate start segment number based on seek position
	startSegmentNum := 0
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		startSegmentNum = b.opts.StartPosition / p.SegmentDuration
	}

	b.args = append(b.args,
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "segment",
		"-segment_format", "mpegts",
		"-segment_list", playlistPath,
		"-segment_list_type", "m3u8",
		"-segment_time", strconv.Itoa(p.SegmentDuration),
		"-segment_start_number", strconv.Itoa(startSegmentNum),
		"-segment_list_flags", "+live",
	)

	// Enable start PTS reporting for accurate seek feedback (requires patched FFmpeg)
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		seekTimeMicroseconds := int64(b.opts.StartPosition) * 1000000
		b.args = append(b.args,
			"-segment_start_pts_report", "1",
			"-segment_seek_time", strconv.FormatInt(seekTimeMicroseconds, 10),
		)
	}

	return b
}

// AddSegmentMuxerOutputFile adds the segment output file pattern.
// Must be called after AddSegmentMuxerOutput().
func (b *Builder) AddSegmentMuxerOutputFile() *Builder {
	segmentPath := filepath.Join(b.opts.OutputDir, SegmentFilenameFormat)
	b.args = append(b.args, segmentPath)
	return b
}

// AddOutputFile adds the output file path.
func (b *Builder) AddOutputFile() *Builder {
	outputPlaylist := filepath.Join(b.opts.OutputDir, "playlist.m3u8")
	b.args = append(b.args, outputPlaylist)
	return b
}
