package streaming

import (
	"testing"
)

func TestByteRange_Length(t *testing.T) {
	tests := []struct {
		name     string
		start    int64
		end      int64
		expected int64
	}{
		{"Single byte", 0, 0, 1},
		{"First 100 bytes", 0, 99, 100},
		{"Middle range", 500, 999, 500},
		{"Large range", 0, 999999, 1000000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ByteRange{Start: tc.start, End: tc.end}
			if r.Length() != tc.expected {
				t.Errorf("expected length %d, got %d", tc.expected, r.Length())
			}
		})
	}
}

func TestByteRange_Fields(t *testing.T) {
	r := ByteRange{Start: 1024, End: 2047}

	if r.Start != 1024 {
		t.Errorf("expected start 1024, got %d", r.Start)
	}
	if r.End != 2047 {
		t.Errorf("expected end 2047, got %d", r.End)
	}
}

func TestVideoInfo_Fields(t *testing.T) {
	info := VideoInfo{
		Codec:           "hevc",
		Width:           3840,
		Height:          2160,
		Bitrate:         50_000_000,
		Duration:        7200.5,
		AudioCodec:      "truehd",
		AudioBitrate:    5_000_000,
		AudioChannels:   8,
		ContainerFormat: "matroska",
		PixelFormat:     "yuv420p10le",
		ColorSpace:      "bt2020nc",
		ColorPrimaries:  "bt2020",
		ColorTransfer:   "smpte2084",
		BitDepth:        10,
		IsHDR:           true,
	}

	if info.Codec != "hevc" {
		t.Errorf("expected codec 'hevc', got %s", info.Codec)
	}
	if info.Width != 3840 {
		t.Errorf("expected width 3840, got %d", info.Width)
	}
	if info.Height != 2160 {
		t.Errorf("expected height 2160, got %d", info.Height)
	}
	if info.Bitrate != 50_000_000 {
		t.Errorf("expected bitrate 50000000, got %d", info.Bitrate)
	}
	if info.Duration != 7200.5 {
		t.Errorf("expected duration 7200.5, got %f", info.Duration)
	}
	if info.AudioCodec != "truehd" {
		t.Errorf("expected audio codec 'truehd', got %s", info.AudioCodec)
	}
	if info.AudioChannels != 8 {
		t.Errorf("expected 8 audio channels, got %d", info.AudioChannels)
	}
	if info.ContainerFormat != "matroska" {
		t.Errorf("expected container 'matroska', got %s", info.ContainerFormat)
	}
	if !info.IsHDR {
		t.Error("expected IsHDR to be true")
	}
	if info.BitDepth != 10 {
		t.Errorf("expected bit depth 10, got %d", info.BitDepth)
	}
}

func TestVideoCodec_Constants(t *testing.T) {
	tests := []struct {
		name     string
		codec    VideoCodec
		expected string
	}{
		{"H264", CodecH264, "h264"},
		{"H265", CodecH265, "h265"},
		{"VP9", CodecVP9, "vp9"},
		{"AV1", CodecAV1, "av1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.codec) != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, tc.codec)
			}
		})
	}
}

func TestVideoCodec_IsValid(t *testing.T) {
	tests := []struct {
		codec VideoCodec
		valid bool
	}{
		{CodecH264, true},
		{CodecH265, true},
		{CodecVP9, true},
		{CodecAV1, true},
		{VideoCodec("unknown"), false},
		{VideoCodec(""), false},
		{VideoCodec("mpeg2"), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.codec), func(t *testing.T) {
			if tc.codec.IsValid() != tc.valid {
				t.Errorf("codec %s: expected valid=%v, got %v", tc.codec, tc.valid, tc.codec.IsValid())
			}
		})
	}
}

func TestVideoCodec_String(t *testing.T) {
	if CodecH264.String() != "h264" {
		t.Errorf("expected 'h264', got %s", CodecH264.String())
	}
	if CodecH265.String() != "h265" {
		t.Errorf("expected 'h265', got %s", CodecH265.String())
	}
}

func TestHardwareAccel_Constants(t *testing.T) {
	tests := []struct {
		name     string
		accel    HardwareAccel
		expected string
	}{
		{"None", AccelNone, "none"},
		{"VAAPI", AccelVAAPI, "vaapi"},
		{"NVENC", AccelNVENC, "nvenc"},
		{"QSV", AccelQSV, "qsv"},
		{"VideoToolbox", AccelVideoToolbox, "videotoolbox"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.accel) != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, tc.accel)
			}
		})
	}
}

func TestHardwareAccel_IsValid(t *testing.T) {
	tests := []struct {
		accel HardwareAccel
		valid bool
	}{
		{AccelNone, true},
		{AccelVAAPI, true},
		{AccelNVENC, true},
		{AccelQSV, true},
		{AccelVideoToolbox, true},
		{HardwareAccel("cuda"), false},
		{HardwareAccel(""), false},
		{HardwareAccel("unknown"), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.accel), func(t *testing.T) {
			if tc.accel.IsValid() != tc.valid {
				t.Errorf("accel %s: expected valid=%v, got %v", tc.accel, tc.valid, tc.accel.IsValid())
			}
		})
	}
}

func TestHardwareAccel_String(t *testing.T) {
	if AccelNVENC.String() != "nvenc" {
		t.Errorf("expected 'nvenc', got %s", AccelNVENC.String())
	}
	if AccelVAAPI.String() != "vaapi" {
		t.Errorf("expected 'vaapi', got %s", AccelVAAPI.String())
	}
}

func TestVideoInfo_HDRMetadata(t *testing.T) {
	// SDR video
	sdrInfo := VideoInfo{
		Codec:          "h264",
		Width:          1920,
		Height:         1080,
		PixelFormat:    "yuv420p",
		ColorSpace:     "bt709",
		ColorPrimaries: "bt709",
		ColorTransfer:  "bt709",
		BitDepth:       8,
		IsHDR:          false,
	}

	if sdrInfo.IsHDR {
		t.Error("SDR video should not be marked as HDR")
	}
	if sdrInfo.BitDepth != 8 {
		t.Errorf("expected 8-bit, got %d", sdrInfo.BitDepth)
	}

	// HDR10 video
	hdr10Info := VideoInfo{
		Codec:          "hevc",
		Width:          3840,
		Height:         2160,
		PixelFormat:    "yuv420p10le",
		ColorSpace:     "bt2020nc",
		ColorPrimaries: "bt2020",
		ColorTransfer:  "smpte2084",
		BitDepth:       10,
		IsHDR:          true,
	}

	if !hdr10Info.IsHDR {
		t.Error("HDR10 video should be marked as HDR")
	}
	if hdr10Info.ColorTransfer != "smpte2084" {
		t.Errorf("expected PQ transfer, got %s", hdr10Info.ColorTransfer)
	}

	// HLG video
	hlgInfo := VideoInfo{
		Codec:          "hevc",
		Width:          3840,
		Height:         2160,
		PixelFormat:    "yuv420p10le",
		ColorSpace:     "bt2020nc",
		ColorPrimaries: "bt2020",
		ColorTransfer:  "arib-std-b67",
		BitDepth:       10,
		IsHDR:          true,
	}

	if !hlgInfo.IsHDR {
		t.Error("HLG video should be marked as HDR")
	}
	if hlgInfo.ColorTransfer != "arib-std-b67" {
		t.Errorf("expected HLG transfer, got %s", hlgInfo.ColorTransfer)
	}
}

func TestVideoInfo_AudioInfo(t *testing.T) {
	// Dolby Atmos
	atmosInfo := VideoInfo{
		AudioCodec:   "truehd",
		AudioBitrate: 5_000_000,
		AudioChannels: 8,
	}

	if atmosInfo.AudioCodec != "truehd" {
		t.Errorf("expected 'truehd', got %s", atmosInfo.AudioCodec)
	}
	if atmosInfo.AudioChannels != 8 {
		t.Errorf("expected 8 channels (7.1), got %d", atmosInfo.AudioChannels)
	}

	// Stereo AAC
	stereoInfo := VideoInfo{
		AudioCodec:   "aac",
		AudioBitrate: 256_000,
		AudioChannels: 2,
	}

	if stereoInfo.AudioChannels != 2 {
		t.Errorf("expected 2 channels (stereo), got %d", stereoInfo.AudioChannels)
	}
}
