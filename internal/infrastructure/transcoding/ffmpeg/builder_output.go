package ffmpeg

import (
	"path/filepath"
	"strconv"
	"strings"
)

// AddHLSOutput adds HLS output settings (without output file - use AddOutputFile() separately).
// Uses MPEG-TS segments for maximum compatibility and fast startup.
// Implements ultra-short initial segment (0.5s) for fast time-to-first-frame,
// followed by longer segments (2-4s) for efficient streaming.
func (b *FFmpegArgsBuilder) AddHLSOutput() *FFmpegArgsBuilder {
	p := b.opts.Profile
	segmentPath := filepath.Join(b.opts.OutputDir, SegmentFilenameFormat)

	// Calculate start segment number based on seek position
	startSegmentNum := 0
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		startSegmentNum = b.opts.StartPosition / p.SegmentDuration
	}

	b.args = append(b.args,
		"-f", "hls",
		// Initial segment of 1s balances fast startup with proper audio initialization.
		// Shorter values (0.5s) can cause incomplete AAC ADTS headers in the first segment
		// after seeking, leading to bufferAppendError in browsers.
		"-hls_init_time", "1",
		"-hls_time", strconv.Itoa(p.SegmentDuration),
		"-hls_playlist_type", "event",
		"-hls_segment_filename", segmentPath,
		// Use MPEG-TS for fast startup (each segment is self-contained)
		"-hls_segment_type", "mpegts",
		// HLS flags:
		// - independent_segments: each segment can be decoded independently
		// - split_by_time: split at exact time boundaries (more predictable segment duration)
		"-hls_flags", "independent_segments+split_by_time",
		"-start_number", strconv.Itoa(startSegmentNum),
	)

	return b
}

// Note: AddForceFirstKeyframe was removed.
// The GOP settings (-g/-keyint_min) already ensure frame 0 is a keyframe since it starts the first GOP.
// Using -force_key_frames added unnecessary overhead as FFmpeg had to evaluate the expression on every frame.

// AddSegmentMuxerOutput adds segment muxer output settings for HLS.
// This uses FFmpeg's segment muxer instead of the HLS muxer, which provides better
// control over segment timing when seeking. Requires patched FFmpeg (viewra-ffmpeg)
// with start_pts tracking fix for correct A/V sync.
//
// The segment muxer with -segment_list_type m3u8 generates HLS-compatible output
// but with proper timestamp handling for mid-stream seeks.
//
// Based on Emby/Jellyfin's approach to HLS segment generation.
func (b *FFmpegArgsBuilder) AddSegmentMuxerOutput() *FFmpegArgsBuilder {
	p := b.opts.Profile
	playlistPath := filepath.Join(b.opts.OutputDir, "playlist.m3u8")

	// Calculate start segment number based on seek position
	startSegmentNum := 0
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		startSegmentNum = b.opts.StartPosition / p.SegmentDuration
	}

	b.args = append(b.args,
		// Eliminate muxer buffering delay for tighter A/V sync
		"-muxdelay", "0",
		"-muxpreload", "0",
		// Use segment muxer instead of HLS muxer for better seek handling
		"-f", "segment",
		"-segment_format", "mpegts",
		"-segment_list", playlistPath,
		"-segment_list_type", "m3u8",
		"-segment_time", strconv.Itoa(p.SegmentDuration),
		"-segment_start_number", strconv.Itoa(startSegmentNum),
		// Use live mode to write accurate segment durations as they complete
		"-segment_list_flags", "+live",
	)

	// Enable start PTS reporting for accurate seek feedback (requires patched FFmpeg)
	// This writes #EXT-X-START-PTS and #EXT-X-START-OFFSET tags to the playlist
	// showing where playback actually starts relative to the requested seek time
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		// Convert seek position from seconds to microseconds for FFmpeg
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
func (b *FFmpegArgsBuilder) AddSegmentMuxerOutputFile() *FFmpegArgsBuilder {
	segmentPath := filepath.Join(b.opts.OutputDir, SegmentFilenameFormat)
	b.args = append(b.args, segmentPath)
	return b
}

// AddOutputFile adds the output file path.
func (b *FFmpegArgsBuilder) AddOutputFile() *FFmpegArgsBuilder {
	outputPlaylist := filepath.Join(b.opts.OutputDir, "playlist.m3u8")
	b.args = append(b.args, outputPlaylist)
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
