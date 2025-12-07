package transcoding

import (
	"testing"
)

// Note: The comprehensive tests for video info functions have been moved to
// internal/infrastructure/transcoding/probe/video_info_test.go
// This file tests the re-exported public API from the parent package.

func TestVideoInfoReExports(t *testing.T) {
	// Verify re-exports work correctly

	// Test IsHDRContent
	if !IsHDRContent("smpte2084", "bt2020") {
		t.Error("IsHDRContent should return true for HDR10")
	}
	if IsHDRContent("bt709", "bt709") {
		t.Error("IsHDRContent should return false for SDR")
	}

	// Test DetectBitDepthFromPixelFormat
	if depth := DetectBitDepthFromPixelFormat("yuv420p"); depth != 8 {
		t.Errorf("DetectBitDepthFromPixelFormat(yuv420p) = %d, want 8", depth)
	}
	if depth := DetectBitDepthFromPixelFormat("yuv420p10le"); depth != 10 {
		t.Errorf("DetectBitDepthFromPixelFormat(yuv420p10le) = %d, want 10", depth)
	}

	// Test SelectBestAudioTrack
	tracks := []AudioTrack{
		{Index: 0, Codec: "aac", Channels: 2},
		{Index: 1, Codec: "ac3", Channels: 6},
	}
	best := SelectBestAudioTrack(tracks)
	if best == nil {
		t.Fatal("SelectBestAudioTrack returned nil")
	}
	if best.Index != 0 {
		t.Errorf("SelectBestAudioTrack selected index %d, want 0", best.Index)
	}

	// Test VideoInfoCache is available
	if VideoInfoCache == nil {
		t.Error("VideoInfoCache should not be nil")
	}
}

func TestVideoInfoTypes(t *testing.T) {
	// Test that type aliases work correctly

	info := &VideoInfo{
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
		Bitrate:         5000000,
		Duration:        120.5,
		AudioCodec:      "aac",
		AudioBitrate:    128000,
		AudioChannels:   2,
		ContainerFormat: "mp4",
		PixelFormat:     "yuv420p",
		ColorTransfer:   "bt709",
		ColorPrimaries:  "bt709",
		BitDepth:        8,
		IsHDR:           false,
		AudioTracks: []AudioTrack{
			{Index: 0, Codec: "aac", Channels: 2, Language: "eng"},
		},
	}

	if info.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", info.Codec)
	}
	if info.Width != 1920 {
		t.Errorf("Width = %d, want 1920", info.Width)
	}
	if len(info.AudioTracks) != 1 {
		t.Errorf("AudioTracks count = %d, want 1", len(info.AudioTracks))
	}
}

func TestAudioTrackType(t *testing.T) {
	// Test AudioTrack type alias

	track := AudioTrack{
		Index:        0,
		Codec:        "aac",
		Channels:     2,
		Bitrate:      128000,
		Language:     "eng",
		Title:        "English Stereo",
		IsCommentary: false,
	}

	if track.Index != 0 {
		t.Errorf("Index = %d, want 0", track.Index)
	}
	if track.Codec != "aac" {
		t.Errorf("Codec = %q, want aac", track.Codec)
	}
	if track.IsCommentary {
		t.Error("IsCommentary should be false")
	}
}
